package io.github.untitled572.pichub.android

import android.graphics.Bitmap
import android.graphics.ImageDecoder
import android.os.SystemClock
import android.security.NetworkSecurityPolicy
import kotlinx.coroutines.CoroutineStart
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.awaitCancellation
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.ensureActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import okhttp3.Call
import okhttp3.Dns
import okhttp3.HttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import okhttp3.ResponseBody
import org.json.JSONArray
import org.json.JSONObject
import java.io.EOFException
import java.io.File
import java.io.IOException
import java.net.InetAddress
import java.net.Proxy
import java.net.SocketException
import java.net.SocketTimeoutException
import java.net.UnknownHostException
import java.time.ZonedDateTime
import java.time.format.DateTimeFormatter
import java.util.Base64
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import javax.net.ssl.SSLException
import kotlin.coroutines.coroutineContext
import kotlin.math.roundToInt

internal data class Picture(
    val id: Long?,
    val distributionRef: String?,
    val original: File,
    val bitmap: Bitmap,
    val mimeType: String,
    val details: PictureDetails
)

internal data class PictureDetails(
    val source: String?,
    val categories: List<String>,
    val width: Int,
    val height: Int,
    val bytes: Long,
    val requestPath: String,
    val responseCode: Int,
    val elapsedMillis: Long
)

private data class PictureResponse(
    val code: Int,
    val url: HttpUrl? = null,
    val id: Long? = null,
    val distributionRef: String? = null,
    val source: String? = null,
    val categories: List<String> = emptyList()
)

internal data class AppTag(val id: String, val name: String)

internal enum class Vote(val parameter: String) {
    Like("like"),
    Dislike("dislike")
}

internal class PicHubClient {
    private val rateLimits = ConcurrentHashMap<String, Long>()
    private val https = OkHttpClient.Builder()
        .callTimeout(45, TimeUnit.SECONDS)
        .connectTimeout(10, TimeUnit.SECONDS)
        .readTimeout(25, TimeUnit.SECONDS)
        .followRedirects(false)
        .retryOnConnectionFailure(false)
        .build()

    private val lan = https.newBuilder()
        .proxy(Proxy.NO_PROXY)
        .dns(object : Dns {
            override fun lookup(hostname: String): List<InetAddress> {
                val addresses = Dns.SYSTEM.lookup(hostname)
                if (addresses.any { !isPrivateAddress(it) }) {
                    throw UnknownHostException("HTTP server is outside the local network")
                }
                return addresses
            }
        })
        .build()

    suspend fun getPicture(connection: PicHubConnection, direction: PictureDirection?, file: File): Picture =
        withContext(Dispatchers.IO) {
            val directory = file.parentFile ?: throw PicHubException("无法创建图片缓存")
            if (!directory.isDirectory && !directory.mkdirs() && !directory.isDirectory) {
                throw PicHubException("无法创建图片缓存")
            }
            val url = connection.nextImage(direction)
            val request = Request.Builder()
                .url(url)
                .header("Accept", "application/json, image/*")
                .header("X-PicHub-Debug-Image", "true")
                .build()
            val started = SystemClock.elapsedRealtime()
            val retry = PictureRetry()
            val metadata = retry.run {
                pictureStage("图片信息") { pictureResponse(connection, request, file) }
            }

            metadata.url?.let {
                retry.run { pictureStage("图片下载") { fetchPicture(connection, it, file) } }
            }
            coroutineContext.ensureActive()

            decodePicture(metadata, direction, file, url, started)
        }

    private suspend fun pictureResponse(connection: PicHubConnection, request: Request, file: File): PictureResponse =
        pictureCall(connection, request) { response ->
        if (!response.isSuccessful) throw pictureHttpFailure(response, metadata = true)
        val body = response.body ?: throw PicHubException("服务器没有返回图片")
        if (body.contentType()?.type == "image") {
            pictureStage("图片下载") { copyPicture(body, file) }
            val debugMetadata = response.header("X-PicHub-Debug-Metadata")?.let { header ->
                runCatching { JSONObject(Base64.getUrlDecoder().decode(header).toString(Charsets.UTF_8)) }.getOrNull()
            }
            return@pictureCall PictureResponse(
                code = response.code,
                source = debugMetadata?.optString("source")?.takeIf { it.isNotBlank() },
                categories = debugMetadata?.optJSONArray("categories").strings()
            )
        }
        if (body.contentType()?.subtype?.contains("json") != true) {
            throw PicHubException("地址没有返回 PicHub 取图响应")
        }
        try {
            val json = JSONObject(body.source().readUtf8AtMost(64 * 1024L))
            val local = json.optString("local_url").ifBlank { json.optString("url") }
            val imageUrl = connection.imageUrl(local.takeIf { it.isNotBlank() }
                ?: throw PicHubException("响应缺少图片地址"))
                ?: throw PicHubException("服务器返回的图片地址无效")
            PictureResponse(
                code = response.code,
                url = imageUrl,
                id = json.optLong("image_id").takeIf { it > 0 },
                distributionRef = json.optString("distribution_ref", "").takeIf { it.isNotBlank() },
                source = (json.opt("source") as? String)?.trim()?.takeIf { it.isNotEmpty() },
                categories = json.optJSONArray("categories").strings()
            )
        } catch (_: org.json.JSONException) {
            throw PicHubException("服务器返回的图片信息无效")
        }
        }

    private fun decodePicture(
        metadata: PictureResponse,
        direction: PictureDirection?,
        file: File,
        url: HttpUrl,
        started: Long
    ): Picture {
        var mime = ""
        var width = 0
        var height = 0
        val bitmap = try {
            ImageDecoder.decodeBitmap(ImageDecoder.createSource(file)) { decoder, info, _ ->
                mime = info.mimeType
                width = info.size.width
                height = info.size.height
                val scale = minOf(1.0, 2560.0 / maxOf(width, height))
                decoder.setTargetSize(
                    (width * scale).roundToInt().coerceAtLeast(1),
                    (height * scale).roundToInt().coerceAtLeast(1)
                )
                decoder.allocator = ImageDecoder.ALLOCATOR_SOFTWARE
            }
        } catch (_: IOException) {
            throw PictureFailure("图片解码失败，请刷新后重试")
        }
        val actual = when {
            bitmap.width > bitmap.height -> PictureDirection.Horizontal
            bitmap.height > bitmap.width -> PictureDirection.Vertical
            else -> null
        }
        if (direction != null && actual != direction) {
            bitmap.recycle()
            val expected = if (direction == PictureDirection.Horizontal) "横图" else "竖图"
            throw PictureFailure("图片方向不匹配：当前请求$expected，图源返回了其他方向的图片")
        }
        return Picture(
            id = metadata.id,
            distributionRef = metadata.distributionRef,
            original = file,
            bitmap = bitmap,
            mimeType = mime,
            details = PictureDetails(
                source = metadata.source,
                categories = metadata.categories,
                width = width,
                height = height,
                bytes = file.length(),
                requestPath = url.encodedPath,
                responseCode = metadata.code,
                elapsedMillis = SystemClock.elapsedRealtime() - started
            )
        )
    }

    suspend fun checkKey(connection: PicHubConnection, key: String) = withContext(Dispatchers.IO) {
        val url = connection.api("auth", "check")
        val request = Request.Builder().url(url).header("Authorization", "Bearer $key").build()
        client(connection, url).newCall(request).withResponse { response ->
            if (response.code != 204) throw feedbackError(response.code)
        }
    }

    suspend fun listTags(connection: PicHubConnection, key: String): List<AppTag> =
        withContext(Dispatchers.IO) {
            val url = connection.api("tags")
            val request = Request.Builder().url(url).header("Authorization", "Bearer $key").build()
            client(connection, url).newCall(request).withResponse { response ->
                if (response.code == 404) throw PicHubException("请更新服务端以使用 App 标签筛选")
                if (!response.isSuccessful) throw feedbackError(response.code)
                val body = response.body ?: throw PicHubException("服务未返回标签列表")
                val payload = JSONObject(body.source().readUtf8AtMost(256 * 1024L))
                val rows = payload.optJSONArray("tags") ?: JSONArray()
                (0 until rows.length()).mapNotNull { index ->
                    val row = rows.optJSONObject(index) ?: return@mapNotNull null
                    val id = row.optString("id")
                    id.takeIf { it.isNotBlank() }?.let { AppTag(it, row.optString("name").ifBlank { id }) }
                }
            }
        }

    suspend fun submit(
        connection: PicHubConnection,
        key: String,
        imageId: Long,
        requestId: String,
        distributionRef: String?,
        vote: Vote?
    ) =
        withContext(Dispatchers.IO) {
            val url = connection.api("feedback")
            val body = JSONObject()
                .put("image_id", imageId)
                .put("request_id", requestId)
                .put("distribution_ref", distributionRef)
                .put("vote", vote?.parameter ?: "none")
                .toString()
                .toRequestBody("application/json".toMediaType())
            val request = Request.Builder().url(url).header("Authorization", "Bearer $key").post(body).build()
            client(connection, url).newCall(request).withResponse { response ->
                if (response.code != 204) throw feedbackError(response.code)
            }
        }

    private suspend fun fetchPicture(connection: PicHubConnection, firstUrl: HttpUrl, file: File) {
        var url = firstUrl
        repeat(6) {
            coroutineContext.ensureActive()
            if (url.username.isNotEmpty() || url.password.isNotEmpty()) {
                throw PicHubException("服务器返回的图片地址无效")
            }
            val request = Request.Builder().url(url).header("Accept", "image/*").build()
            val redirect = pictureCall(connection, request) { response ->
                if (response.code in setOf(301, 302, 303, 307, 308)) {
                    response.header("Location")?.let { url.resolve(it) }
                        ?: throw PicHubException("图片重定向地址无效")
                } else {
                    if (!response.isSuccessful) throw pictureHttpFailure(response, metadata = false)
                    val body = response.body ?: throw PicHubException("服务器没有返回图片")
                    copyPicture(body, file)
                    null
                }
            } ?: return
            url = redirect
        }
        throw PicHubException("图片重定向次数过多")
    }

    fun validateAddress(connection: PicHubConnection) {
        client(connection, connection.enteredUrl)
    }

    private fun client(connection: PicHubConnection, url: HttpUrl): OkHttpClient {
        if (url.isHttps) return https
        if (!connection.matchesOrigin(url)) {
            throw PicHubException("HTTP 图片链接必须来自所配置的局域网服务")
        }
        if (!NetworkSecurityPolicy.getInstance().isCleartextTrafficPermitted(url.host)) {
            throw PicHubException("此地址请使用 HTTPS，或用受支持的局域网 .local 域名")
        }
        return lan
    }

    private suspend fun copyPicture(body: ResponseBody, file: File) {
        val maxBytes = 64L * 1024 * 1024
        if (body.contentLength() > maxBytes) throw PicHubException("图片超过 64 MB")
        body.byteStream().use { input ->
            file.outputStream().buffered().use { output ->
                val buffer = ByteArray(32 * 1024)
                var total = 0L
                while (true) {
                    coroutineContext.ensureActive()
                    val count = input.read(buffer)
                    if (count < 0) break
                    total += count
                    if (total > maxBytes) throw PicHubException("图片超过 64 MB")
                    output.write(buffer, 0, count)
                }
            }
        }
    }

    private suspend fun <T> pictureCall(
        connection: PicHubConnection,
        request: Request,
        consume: suspend (Response) -> T
    ): T {
        val origin = "${request.url.scheme}://${request.url.host}:${request.url.port}"
        val now = SystemClock.elapsedRealtime()
        rateLimits.entries.removeIf { it.value <= now }
        val remaining = rateLimits[origin]?.minus(now) ?: 0L
        if (remaining > 0) throw PictureFailure(rateLimitMessage(remaining))
        return client(connection, request.url).newCall(request).withResponse { response ->
            if (response.code == 429) {
                val wait = retryAfterMillis(response.header("Retry-After")) ?: 1_000L
                if (wait > 0) rateLimits.merge(origin, SystemClock.elapsedRealtime() + wait, ::maxOf)
            }
            consume(response)
        }
    }

    private fun pictureHttpFailure(response: Response, metadata: Boolean): PictureFailure {
        val code = response.code
        val stage = if (metadata) "图片信息获取失败" else "图片下载失败"
        val errorBody = if (metadata && code == 503) response.peekBody(1024).string() else ""
        val noSources = errorBody.contains("no available sources", ignoreCase = true)
        val errorCode = runCatching { JSONObject(errorBody).optString("code") }.getOrNull()
        val serverTimedOut = errorCode == "distribution_timeout"
        val debugFailed = errorCode == "debug_distribution_failed"
        val wait = retryAfterMillis(response.header("Retry-After"))
        val message = when {
            code == 429 -> "$stage，${rateLimitMessage(wait ?: 1_000L)}"
            noSources -> "服务暂时没有符合当前标签和方向的图源"
            serverTimedOut -> "服务等待上游图源超时，请稍后刷新"
            debugFailed -> "调试图源本次取图失败，请检查图源和分支"
            errorCode == "distribution_mode_changed" -> "图源调试状态正在切换"
            metadata && code == 503 -> "服务暂时无法取图（HTTP 503）"
            else -> "$stage（HTTP $code）"
        }
        val retryDelay = when {
            noSources || serverTimedOut || debugFailed -> null
            code == 429 -> wait ?: 1_000L
            code in setOf(408, 500, 502, 503, 504) -> wait ?: 350L
            else -> null
        }
        return PictureFailure(message, retryDelay)
    }

    private fun feedbackError(code: Int): PicHubException = PicHubException(when (code) {
        401 -> "App 密钥无效或已撤销"
        404 -> "当前服务版本没有安卓赞踩接口"
        409 -> "这次反馈已提交，请刷新后继续"
        410 -> "当前图片的服务端缓存已过期，请刷新后再反馈"
        429 -> "反馈太频繁，请稍后重试"
        else -> "反馈请求失败（HTTP $code）"
    })
}

private class PictureFailure(message: String, val retryDelay: Long? = null) : Exception(message)

private class PictureRetry {
    private var used = false

    suspend fun <T> run(fetch: suspend () -> T): T {
        try {
            return fetch()
        } catch (error: PictureFailure) {
            val wait = error.retryDelay ?: throw error
            if (used || wait > 3_000L) throw error
            used = true
            delay(wait)
            return fetch()
        }
    }
}

private suspend fun <T> pictureStage(stage: String, fetch: suspend () -> T): T {
    try {
        return fetch()
    } catch (error: PicHubException) {
        throw PictureFailure("${stage}失败，${error.message}")
    } catch (error: IOException) {
        coroutineContext.ensureActive()
        val reason = error.userMessage("连接中断或读取失败")
        val retry = error is SocketTimeoutException || error is SocketException || error is EOFException || error.cause is EOFException
        throw PictureFailure("${stage}失败，$reason", if (retry) 350L else null)
    }
}

internal fun retryAfterMillis(header: String?, nowMillis: Long = System.currentTimeMillis()): Long? {
    val value = header?.trim()?.takeIf { it.isNotEmpty() } ?: return null
    value.toLongOrNull()?.let { return it.coerceIn(0L, 31_536_000L) * 1_000L }
    return runCatching {
        val deadline = ZonedDateTime.parse(value, DateTimeFormatter.RFC_1123_DATE_TIME).toInstant().toEpochMilli()
        (deadline - nowMillis).coerceIn(0L, 31_536_000_000L)
    }.getOrNull()
}

private fun rateLimitMessage(wait: Long) = "请求太频繁，请在 ${(wait + 999L) / 1_000L} 秒后重试"

private suspend fun <T> Call.withResponse(consume: suspend (Response) -> T): T = coroutineScope {
    val finished = AtomicBoolean(false)
    val cancelCall = launch(start = CoroutineStart.UNDISPATCHED) {
        try {
            awaitCancellation()
        } finally {
            if (!finished.get()) this@withResponse.cancel()
        }
    }
    try {
        execute().use { consume(it) }
    } finally {
        finished.set(true)
        cancelCall.cancel()
    }
}

private fun JSONArray?.strings(): List<String> = if (this == null) emptyList() else
    (0 until minOf(length(), 12)).mapNotNull { opt(it) as? String }.filter { it.isNotBlank() }

private fun okio.BufferedSource.readUtf8AtMost(limit: Long): String {
    if (request(limit + 1)) throw PicHubException("服务器响应过大")
    return readUtf8()
}

internal fun Throwable.userMessage(fallback: String): String = when (this) {
    is PicHubException, is PictureFailure -> message ?: fallback
    is SocketTimeoutException -> "连接超时，请稍后重试"
    is UnknownHostException -> "找不到服务器，请检查地址和局域网连接"
    is SSLException -> "HTTPS 证书验证失败，请检查服务器证书"
    else -> fallback
}
