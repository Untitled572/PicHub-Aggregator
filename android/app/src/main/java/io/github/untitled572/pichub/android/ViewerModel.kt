package io.github.untitled572.pichub.android

import android.app.Application
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import java.io.File
import java.util.UUID

internal data class DisplayedPicture(
    val picture: Picture,
    val requestId: String = UUID.randomUUID().toString(),
    val attemptedVote: Vote? = null,
    val acceptedVote: Vote? = null,
    val saved: Boolean = false
)

internal class ViewerModel(application: Application) : AndroidViewModel(application) {
    private val settings = ConnectionPreferences(application)
    private val client = PicHubClient()
    private val pictureFolder = File(application.cacheDir, "pichub-images")
    private var connection = runCatching { PicHubConnection.parse(settings.address) }.getOrNull()
    private var key = settings.loadKey()

    var showSettings by mutableStateOf(connection == null)
        private set
    var picture by mutableStateOf<DisplayedPicture?>(null)
        private set
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

    private var direction: PictureDirection? = null
    private var loadJob: Job? = null
    private var loadGeneration = 0
    private var loadingDirection: PictureDirection? = null
    private var testJob: Job? = null
    private var voteJob: Job? = null
    private var savingFile: File? = null

    init {
        viewModelScope.launch(Dispatchers.IO) {
            val cutoff = System.currentTimeMillis() - 24 * 60 * 60 * 1000L
            pictureFolder.listFiles()?.filter { it.isFile && it.lastModified() < cutoff }?.forEach { it.delete() }
        }
    }

    fun setDirection(newDirection: PictureDirection) {
        if (direction == newDirection) return
        direction = newDirection
        if (!showSettings) refresh()
    }

    fun refresh() {
        val server = connection ?: return
        val requestedDirection = direction ?: return
        val generation = ++loadGeneration
        loadJob?.cancel()
        loading = true
        loadingDirection = requestedDirection
        loadingError = null
        loadJob = viewModelScope.launch {
            val file = newPictureFile()
            var retained = false
            try {
                val next = client.getPicture(server, requestedDirection, file)
                val previous = picture?.picture?.original
                picture = DisplayedPicture(next)
                retained = true
                if (previous != savingFile) previous?.delete()
            } catch (error: CancellationException) {
                throw error
            } catch (error: Exception) {
                if (generation == loadGeneration) {
                    val message = error.userMessage("取图失败，请检查连接后重试")
                    if (picture == null) loadingError = message else notice = message
                }
            } finally {
                if (!retained) file.delete()
                if (generation == loadGeneration) loading = false
            }
        }
    }

    fun save() {
        val shown = picture ?: return
        if (saving || shown.saved) return
        saving = true
        savingFile = shown.picture.original
        viewModelScope.launch {
            try {
                saveToGallery(getApplication<Application>().contentResolver, shown.picture)
                val current = picture
                if (current?.requestId == shown.requestId) picture = current.copy(saved = true)
                notice = "已保存到相册的 PicHub 文件夹"
            } catch (error: CancellationException) {
                throw error
            } catch (error: Exception) {
                notice = error.userMessage("保存失败，请检查手机存储空间")
            } finally {
                saving = false
                savingFile = null
                if (picture?.requestId != shown.requestId) shown.picture.original.delete()
            }
        }
    }

    fun vote(vote: Vote) {
        val shown = picture ?: return
        val server = connection ?: return
        val connectedKey = key
        val imageId = shown.picture.id ?: return
        if (connectedKey.isEmpty() || voting || shown.acceptedVote != null) return
        if (shown.attemptedVote != null && shown.attemptedVote != vote) return

        picture = shown.copy(attemptedVote = vote)
        voting = true
        voteJob = viewModelScope.launch {
            try {
                client.submit(server, connectedKey, imageId, shown.requestId, vote)
                val current = picture
                if (current?.requestId == shown.requestId) picture = current.copy(acceptedVote = vote)
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
        draftFeedbackEnabled = feedbackEnabled
        draftKey = key
        connectionResult = emptyList()
        showSettings = true
    }

    fun closeSettings() {
        clearTest()
        showSettings = false
        val currentDirection = if (loading) loadingDirection else picture?.picture?.let(::pictureDirection)
        if (connection != null && currentDirection != direction) refresh()
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
        val testDirection = direction ?: PictureDirection.Vertical
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
        val server = parseDraft() ?: return
        val newKey = if (draftFeedbackEnabled) draftKey.trim() else ""
        if (draftFeedbackEnabled && !APP_KEY_PATTERN.matches(newKey)) {
            clearTest()
            connectionResult = listOf("请填写从 PicHub 控制台生成的 App 密钥")
            return
        }
        try {
            settings.save(server.enteredUrl.toString(), newKey)
        } catch (_: Exception) {
            connectionResult = listOf("无法保存连接设置，请重试")
            return
        }

        val addressChanged = connection?.enteredUrl != server.enteredUrl
        if (addressChanged || key != newKey) voteJob?.cancel()
        connection = server
        key = newKey
        draftKey = newKey
        if (addressChanged) {
            val previous = picture?.picture?.original
            picture = null
            if (previous != savingFile) previous?.delete()
            refresh()
        }
        closeSettings()
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

    private fun newPictureFile() = File(pictureFolder, UUID.randomUUID().toString())

    private fun pictureDirection(image: Picture): PictureDirection =
        if (image.bitmap.width > image.bitmap.height) PictureDirection.Horizontal else PictureDirection.Vertical

    override fun onCleared() {
        val currentFile = picture?.picture?.original
        if (currentFile != savingFile) currentFile?.delete()
        picture = null
        super.onCleared()
    }

    private companion object {
        val APP_KEY_PATTERN = Regex("ph_app_[A-Za-z0-9_-]{43}")
    }
}
