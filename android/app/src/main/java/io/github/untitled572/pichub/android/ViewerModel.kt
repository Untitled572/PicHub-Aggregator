package io.github.untitled572.pichub.android

import android.app.Application
import android.net.Uri
import android.provider.DocumentsContract
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Deferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.async
import kotlinx.coroutines.launch
import okhttp3.HttpUrl
import java.io.File
import java.util.UUID

internal data class DisplayedPicture(
    val picture: Picture,
    val requestId: String = UUID.randomUUID().toString(),
    val pendingVote: PendingVote? = null,
    val acceptedVote: Vote? = picture.feedback,
    val saved: Boolean = false
)

internal data class PendingVote(val button: Vote, val desired: Vote?, val requestId: String = UUID.randomUUID().toString())

internal class ViewerModel(application: Application) : AndroidViewModel(application) {
    private val settings = ConnectionPreferences(application)
    private val resolver = application.contentResolver
    private val client = PicHubClient()
    private val pictureFolder = File(application.cacheDir, "pichub-images")
    private var connection = runCatching { PicHubConnection.parse(settings.address) }.getOrNull()
    private var key = settings.loadKey()

    var showSettings by mutableStateOf(connection == null)
        private set
    private var latest by mutableStateOf<DisplayedPicture?>(null)
    private var previous by mutableStateOf<DisplayedPicture?>(null)
    private var showingPrevious by mutableStateOf(false)
    val picture: DisplayedPicture? get() = if (showingPrevious) previous else latest
    var loading by mutableStateOf(false)
        private set
    var loadingError by mutableStateOf<String?>(null)
        private set
    var saving by mutableStateOf(false)
        private set
    var voting by mutableStateOf(false)
        private set
    var notice by mutableStateOf<String?>(null)
        private set

    var viewerOptions by mutableStateOf(settings.loadViewerOptions())
        private set
    var draftOptions by mutableStateOf(viewerOptions)
        private set
    var draftAddress by mutableStateOf(settings.address)
        private set
    var draftFeedbackEnabled by mutableStateOf(key.isNotEmpty())
        private set
    var draftKey by mutableStateOf(key)
        private set
    var testing by mutableStateOf(false)
        private set
    var connectionResult by mutableStateOf<List<String>>(emptyList())
        private set

    val configured: Boolean get() = connection != null
    val feedbackEnabled: Boolean get() = key.isNotEmpty()
    val canShowPrevious: Boolean get() = !loading && !showingPrevious && previous != null
    val draftTags: List<String>
        get() = runCatching { PicHubConnection.parse(draftAddress).tags }.getOrDefault(emptyList())
    val draftScopeLabel: String
        get() {
            val tags = draftTags
            if ("__uncategorized__" in tags) return "全部 Tags 标签"
            return tags.takeIf { it.isNotEmpty() }?.joinToString(" · ") ?: "沿用服务器默认范围"
        }
    val canChooseTags: Boolean
        get() = draftFeedbackEnabled && APP_KEY_PATTERN.matches(draftKey.trim()) && draftAddress.isNotBlank()

    private var direction: PictureDirection? = null
    private var loadJob: Job? = null
    private var loadGeneration = 0
    private var displayedScope: HttpUrl? = null
    private var activeLoad: PictureLoad? = null
    private var prefetched: PictureLoad? = null
    private var testJob: Job? = null
    private var voteJob: Job? = null
    private var savingFile: File? = null
    private var savingTree: Uri? = null
    private var selectedTree: Uri? = null

    init {
        viewModelScope.launch(Dispatchers.IO) {
            val cutoff = System.currentTimeMillis() - 24 * 60 * 60 * 1000L
            pictureFolder.listFiles()?.filter { it.isFile && it.lastModified() < cutoff }?.forEach { it.delete() }
        }
    }

    fun setDirection(newDirection: PictureDirection) {
        if (direction == newDirection) return
        val before = pictureRequest()?.url
        direction = newDirection
        if (!showSettings && before != pictureRequest()?.url) {
            cancelLoads()
            refresh()
        }
    }

    fun nextPicture() {
        if (loading) return
        if (showingPrevious) showingPrevious = false else refresh()
    }

    fun previousPicture() {
        if (canShowPrevious) showingPrevious = true
    }

    fun refresh() {
        if (loading || showSettings) return
        val request = pictureRequest() ?: return
        val generation = ++loadGeneration
        val prepared = prefetched
        prefetched = null
        val load = if (prepared?.url == request.url) prepared else {
            prepared?.discard()
            startLoad(request)
        }
        activeLoad = load
        loading = true
        loadingError = null
        loadJob = viewModelScope.launch {
            var retained = false
            try {
                val next = load.result.await().getOrThrow()
                val staleFiles = listOfNotNull(latest?.picture?.original, previous?.picture?.original)
                previous = latest?.takeIf { displayedScope == request.url }
                latest = DisplayedPicture(next)
                displayedScope = request.url
                showingPrevious = false
                retained = true
                staleFiles.forEach(::removeUnusedPicture)
                prefetchNext()
            } catch (error: CancellationException) {
                throw error
            } catch (error: Exception) {
                if (generation == loadGeneration) {
                    val message = error.userMessage("取图失败，请检查连接后重试")
                    if (picture == null) loadingError = message else notice = message
                }
            } finally {
                if (!retained) load.discard()
                if (generation == loadGeneration) {
                    loading = false
                    activeLoad = null
                }
            }
        }
    }

    private fun pictureRequest(): PictureRequest? {
        val server = connection ?: return null
        val screen = direction ?: return null
        return PictureRequest(server, server.pictureDirection(screen))
    }

    private fun prefetchNext() {
        if (showSettings || prefetched != null) return
        val request = pictureRequest() ?: return
        prefetched = startLoad(request)
    }

    private fun startLoad(request: PictureRequest): PictureLoad {
        val file = newPictureFile()
        val result = viewModelScope.async {
            try {
                Result.success(client.getPicture(request.server, request.direction, file))
            } catch (error: CancellationException) {
                file.delete()
                throw error
            } catch (error: Exception) {
                file.delete()
                Result.failure(error)
            }
        }
        return PictureLoad(request.url, file, result)
    }

    private fun cancelLoads() {
        loadGeneration++
        loadJob?.cancel()
        activeLoad?.discard()
        prefetched?.discard()
        activeLoad = null
        prefetched = null
        loading = false
    }

    fun save() {
        val shown = picture ?: return
        if (saving || shown.saved) return
        val tree = viewerOptions.saveTree
        saving = true
        savingFile = shown.picture.original
        savingTree = tree
        viewModelScope.launch {
            try {
                savePicture(resolver, shown.picture, tree)
                if (viewerOptions.saveTree == tree) {
                    updatePicture(shown.requestId) { it.copy(saved = true) }
                }
                notice = "已保存到 " + saveDirectoryLabel(tree)
            } catch (error: CancellationException) {
                throw error
            } catch (error: Exception) {
                val fallback = if (tree == null) "保存失败，请检查手机存储空间"
                else "无法写入所选目录，请在设置中重新选择"
                notice = error.userMessage(fallback)
            } finally {
                saving = false
                savingFile = null
                savingTree = null
                releaseUnusedFolder(tree)
                removeUnusedPicture(shown.picture.original)
            }
        }
    }

    fun canVote(vote: Vote): Boolean {
        val shown = picture ?: return false
        if (!feedbackEnabled || voting || !shown.picture.feedbackSupported || shown.picture.id == null ||
            displayedScope != pictureRequest()?.url
        ) return false
        if (shown.picture.distributionRef == null && shown.acceptedVote != null) return false
        return shown.pendingVote == null || shown.pendingVote.button == vote
    }

    fun vote(vote: Vote) {
        val shown = picture ?: return
        val server = connection ?: return
        val connectedKey = key
        val imageId = shown.picture.id ?: return
        if (!canVote(vote)) return
        val pending = shown.pendingVote ?: PendingVote(vote, vote.takeUnless { it == shown.acceptedVote })

        updatePicture(shown.requestId) { it.copy(pendingVote = pending) }
        voting = true
        voteJob = viewModelScope.launch {
            try {
                val accepted = client.submit(server, connectedKey, imageId, pending.requestId, shown.picture.distributionRef, pending.desired)
                updatePicture(shown.requestId) { it.copy(pendingVote = null, acceptedVote = accepted) }
            } catch (error: CancellationException) {
                throw error
            } catch (error: Exception) {
                notice = error.userMessage("反馈状态暂未确认，可再次点击同一个按钮重试")
            } finally {
                voting = false
            }
        }
    }

    fun openSettings() {
        draftAddress = settings.address
        draftOptions = viewerOptions
        selectedTree = null
        draftFeedbackEnabled = feedbackEnabled
        draftKey = key
        connectionResult = emptyList()
        showSettings = true
        cancelLoads()
    }

    private fun finishSettings() {
        clearTest()
        showSettings = false
        val request = pictureRequest() ?: return
        if (displayedScope != request.url) refresh() else prefetchNext()
    }

    fun updateAddress(address: String) {
        draftAddress = address
        val savedFeedbackUrl = connection?.api("feedback")
        if (savedFeedbackUrl != null && key.isNotEmpty() && draftKey == key) {
            val next = runCatching { PicHubConnection.parse(address) }.getOrNull()
            if (next != null && next.api("feedback") != savedFeedbackUrl) draftKey = ""
        }
        clearTest()
    }

    suspend fun listDraftTags(): List<AppTag> {
        val server = PicHubConnection.parse(draftAddress)
        val draftToken = draftKey.trim()
        if (!draftFeedbackEnabled || !APP_KEY_PATTERN.matches(draftToken)) {
            throw PicHubException("请先填写 App 密钥")
        }
        return client.listTags(server, draftToken)
    }

    fun selectDraftTags(tags: List<String>) {
        val server = parseDraft() ?: return
        updateAddress(server.withTags(tags))
    }

    fun updateSaveTree(tree: Uri?) {
        if (tree == null || (tree.scheme == "content" && DocumentsContract.isTreeUri(tree))) {
            selectedTree = tree
            draftOptions = draftOptions.copy(saveTree = tree)
        }
    }

    fun updateFillScreen(enabled: Boolean) {
        draftOptions = draftOptions.copy(fillScreen = enabled)
    }

    fun updateShowDetails(enabled: Boolean) {
        draftOptions = draftOptions.copy(showDetails = enabled)
    }

    fun updateFeedbackEnabled(enabled: Boolean) {
        draftFeedbackEnabled = enabled
        clearTest()
    }

    fun updateKey(value: String) {
        draftKey = value
        clearTest()
    }

    fun testConnection() {
        val server = parseDraft() ?: return
        val testingFeedback = draftFeedbackEnabled
        val checkedKey = if (testingFeedback) draftKey.trim() else ""
        val testDirection = server.pictureDirection(direction ?: PictureDirection.Vertical)
        clearTest()
        testing = true
        testJob = viewModelScope.launch {
            val file = newPictureFile()
            try {
                val results = mutableListOf<String>()
                try {
                    val preview = client.getPicture(server, testDirection, file)
                    results.add("取图正常")
                    if (testingFeedback && preview.id == null) {
                        results.add("这次响应没有图片 ID，请检查服务端缓存代理模式")
                    }
                } catch (error: CancellationException) {
                    throw error
                } catch (error: Exception) {
                    results.add(error.userMessage("取图失败，请检查网络和链接"))
                }
                if (checkedKey.isNotEmpty()) {
                    try {
                        client.checkKey(server, checkedKey)
                        results.add("赞踩密钥有效")
                    } catch (error: CancellationException) {
                        throw error
                    } catch (error: Exception) {
                        results.add(error.userMessage("赞踩权限测试失败"))
                    }
                } else if (testingFeedback) {
                    results.add("填写 App 密钥后可测试赞踩权限")
                }
                connectionResult = results
            } finally {
                file.delete()
            }
            testing = false
        }
    }

    fun applySettings() {
        if (connection == null && draftAddress.isBlank()) {
            finishSettings()
            return
        }
        val server = parseDraft()
        if (server == null) {
            notice = connectionResult.firstOrNull()
            return
        }
        val newKey = if (draftFeedbackEnabled) draftKey.trim() else ""
        if (draftFeedbackEnabled && !APP_KEY_PATTERN.matches(newKey)) {
            clearTest()
            showSettingsError("请填写从 PicHub 控制台生成的 App 密钥")
            return
        }
        val previousTree = viewerOptions.saveTree
        val nextTree = draftOptions.saveTree
        var acquiredPermission = false
        try {
            if (nextTree != null && (nextTree != previousTree || nextTree == selectedTree)) {
                if (resolver.persistedUriPermissions.none { it.uri == nextTree && it.isWritePermission }) {
                    resolver.takePersistableUriPermission(nextTree, folderPermissionFlags)
                    acquiredPermission = true
                }
            }
        } catch (_: SecurityException) {
            showSettingsError("无法授权所选目录，请重新选择")
            return
        }
        try {
            settings.save(server.enteredUrl.toString(), newKey, draftOptions)
        } catch (_: Exception) {
            if (acquiredPermission) releaseUnusedFolder(nextTree)
            showSettingsError("无法保存设置，请重试")
            return
        }
        viewerOptions = draftOptions
        if (previousTree != nextTree) {
            latest = latest?.copy(saved = false)
            previous = previous?.copy(saved = false)
            releaseUnusedFolder(previousTree)
        }

        val addressChanged = connection?.enteredUrl != server.enteredUrl
        if (addressChanged || key != newKey) voteJob?.cancel()
        connection = server
        key = newKey
        draftKey = newKey
        finishSettings()
    }

    private fun showSettingsError(message: String) {
        connectionResult = listOf(message)
        notice = message
    }

    fun dismissNotice(value: String?) {
        if (notice == value) notice = null
    }

    private fun parseDraft(): PicHubConnection? = try {
        val server = PicHubConnection.parse(draftAddress)
        client.validateAddress(server)
        server
    } catch (error: PicHubException) {
        clearTest()
        connectionResult = listOf(error.userMessage("连接设置无效"))
        null
    }

    private fun clearTest() {
        testJob?.cancel()
        testing = false
        connectionResult = emptyList()
    }

    private fun releaseUnusedFolder(tree: Uri?) {
        if (tree == null || tree == viewerOptions.saveTree || tree == savingTree) return
        runCatching { resolver.releasePersistableUriPermission(tree, folderPermissionFlags) }
    }

    private fun updatePicture(id: String, update: (DisplayedPicture) -> DisplayedPicture) {
        latest?.takeIf { it.requestId == id }?.let { latest = update(it) }
        previous?.takeIf { it.requestId == id }?.let { previous = update(it) }
    }

    private fun clearPictures() {
        val files = listOfNotNull(latest?.picture?.original, previous?.picture?.original)
        latest = null
        previous = null
        showingPrevious = false
        files.forEach(::removeUnusedPicture)
    }

    private fun removeUnusedPicture(file: File) {
        if (file != savingFile && file != latest?.picture?.original && file != previous?.picture?.original) {
            file.delete()
        }
    }

    private fun newPictureFile() = File(pictureFolder, UUID.randomUUID().toString())

    override fun onCleared() {
        cancelLoads()
        clearPictures()
        super.onCleared()
    }

    private data class PictureRequest(val server: PicHubConnection, val direction: PictureDirection?) {
        val url: HttpUrl = server.nextImage(direction)
    }

    private class PictureLoad(val url: HttpUrl, val file: File, val result: Deferred<Result<Picture>>) {
        fun discard() {
            result.invokeOnCompletion { file.delete() }
            result.cancel()
        }
    }

    private companion object {
        val APP_KEY_PATTERN = Regex("ph_app_[A-Za-z0-9_-]{43}")
    }
}
