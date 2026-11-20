package io.github.untitled572.pichub.android

import android.graphics.Bitmap
import android.graphics.ImageDecoder
import android.security.NetworkSecurityPolicy
import kotlinx.coroutines.CoroutineStart
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.awaitCancellation
import kotlinx.coroutines.coroutineScope
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
import org.json.JSONObject
import java.io.File
import java.io.IOException
import java.net.InetAddress
import java.net.Proxy
import java.net.SocketTimeoutException
import java.net.UnknownHostException
import java.util.concurrent.TimeUnit
import javax.net.ssl.SSLException
import kotlin.coroutines.coroutineContext
import kotlin.math.roundToInt

internal data class Picture(
    val id: Long?,
    val original: File,
    val bitmap: Bitmap,
    val mimeType: String
)

internal enum class Vote(val parameter: String) {
    Like("like"),
    Dislike("dislike")
}

internal class PicHubClient {
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

    suspend fun getPicture(connection: PicHubConnection, direction: PictureDirection, file: File): Picture =
        withContext(Dispatchers.IO) {
            val directory = file.parentFile ?: throw PicHubException("无法创建图片缓存")
            if (!directory.isDirectory && !directory.mkdirs() && !directory.isDirectory) {
                throw PicHubException("无法创建图片缓存")
            }
            val url = connection.nextImage(direction)
            val request = Request.Builder().url(url).header("Accept", "application/json, image/*").build()
            val metadata = client(connection, url).newCall(request).withResponse { response ->
                if (!response.isSuccessful) throw requestError(response.code)
                val body = response.body ?: throw PicHubException("服务器没有返回图片")
                if (body.contentType()?.type == "image") {
                    copyPicture(body, file)
                    return@withResponse null
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
                    val id = json.optLong("image_id").takeIf { it > 0 }
                    imageUrl to id
                } catch (_: org.json.JSONException) {
                    throw PicHubException("服务器返回的图片信息无效")
                }
            }

            if (metadata != null) {
                fetchPicture(connection, metadata.first, file)
            }
            coroutineContext.ensureActive()

            var mime = ""
            val bitmap = try {
                ImageDecoder.decodeBitmap(ImageDecoder.createSource(file)) { decoder, info, _ ->
                    mime = info.mimeType
                    val width = info.size.width
                    val height = info.size.height
                    val scale = minOf(1.0, 2560.0 / maxOf(width, height))
                    decoder.setTargetSize(
                        (width * scale).roundToInt().coerceAtLeast(1),
                        (height * scale).roundToInt().coerceAtLeast(1)
                    )
                    decoder.allocator = ImageDecoder.ALLOCATOR_SOFTWARE
                }
            } catch (_: IOException) {
                throw PicHubException("无法解码这张图片")
            }
            val actual = when {
                bitmap.width > bitmap.height -> PictureDirection.Horizontal
                bitmap.height > bitmap.width -> PictureDirection.Vertical
                else -> null
            }
            if (actual != direction) {
                throw PicHubException("图源返回了其他方向的图片，请重试或检查服务端缓存代理设置")
            }
            Picture(metadata?.second, file, bitmap, mime)
        }

    suspend fun checkKey(connection: PicHubConnection, key: String) = withContext(Dispatchers.IO) {
        val url = connection.api("auth", "check")
        val request = Request.Builder().url(url).header("Authorization", "Bearer $key").build()
        client(connection, url).newCall(request).withResponse { response ->
            if (response.code != 204) throw feedbackError(response.code)
        }
    }

    suspend fun submit(connection: PicHubConnection, key: String, imageId: Long, requestId: String, vote: Vote) =
        withContext(Dispatchers.IO) {
            val url = connection.api("feedback")
            val body = JSONObject()
                .put("image_id", imageId)
                .put("request_id", requestId)
                .put("vote", vote.parameter)
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
            val redirect = client(connection, url).newCall(request).withResponse { response ->
                if (response.code in setOf(301, 302, 303, 307, 308)) {
                    response.header("Location")?.let { url.resolve(it) }
                        ?: throw PicHubException("图片重定向地址无效")
                } else {
                    if (!response.isSuccessful) throw requestError(response.code)
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

    private fun requestError(code: Int): PicHubException = PicHubException(when (code) {
        404 -> "找不到取图端点或图片，请检查链接后重试"
        429 -> "请求太频繁，请稍后刷新"
        503 -> "服务暂时没有符合当前方向的图片"
        else -> "取图失败（HTTP $code）"
    })

    private fun feedbackError(code: Int): PicHubException = PicHubException(when (code) {
        401 -> "App 密钥无效或已撤销"
        404 -> "当前服务版本没有安卓赞踩接口"
        409 -> "这次反馈已提交，请刷新后继续"
        410 -> "当前图片的服务端缓存已过期，请刷新后再反馈"
        429 -> "反馈太频繁，请稍后重试"
        else -> "反馈请求失败（HTTP $code）"
    })
}

private suspend fun <T> Call.withResponse(consume: suspend (Response) -> T): T = coroutineScope {
    val cancelCall = launch(start = CoroutineStart.UNDISPATCHED) {
        try {
            awaitCancellation()
        } finally {
            this@withResponse.cancel()
        }
    }
    try {
        execute().use { consume(it) }
    } finally {
        cancelCall.cancel()
    }
}

private fun okio.BufferedSource.readUtf8AtMost(limit: Long): String {
    if (request(limit + 1)) throw PicHubException("服务器响应过大")
    return readUtf8()
}

internal fun Throwable.userMessage(fallback: String): String = when (this) {
    is PicHubException -> message ?: fallback
    is SocketTimeoutException -> "连接超时，请稍后重试"
    is UnknownHostException -> "找不到服务器，请检查地址和局域网连接"
    is SSLException -> "HTTPS 证书验证失败，请检查服务器证书"
    else -> fallback
}
