package io.github.untitled572.pichub.android

import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull
import java.net.InetAddress

internal enum class PictureDirection(val parameter: String) {
    Horizontal("horizontal"),
    Vertical("vertical")
}

internal class PicHubConnection private constructor(
    val enteredUrl: HttpUrl,
    private val baseUrl: HttpUrl,
    private val distributionUrl: HttpUrl
) {
    val tags: List<String> =
        distributionUrl.queryParameter("category")?.split(',')?.filter { it.isNotBlank() }.orEmpty()

    fun withTags(tags: List<String>): String = enteredUrl.newBuilder()
        .removeAllQueryParameters("category")
        .apply { if (tags.isNotEmpty()) addQueryParameter("category", tags.joinToString(",")) }
        .build().toString()

    fun nextImage(direction: PictureDirection): HttpUrl = distributionUrl.newBuilder()
        .setQueryParameter("format", "json")
        .setQueryParameter("orientation", direction.parameter)
        .build()

    fun api(vararg segments: String): HttpUrl = baseUrl.newBuilder()
        .addPathSegments("api/app")
        .apply { segments.forEach { addPathSegment(it) } }
        .build()

    fun imageUrl(value: String): HttpUrl? {
        return if (value.startsWith("/images/")) {
            baseUrl.resolve(value.removePrefix("/"))
        } else {
            baseUrl.resolve(value)
        }
    }

    fun matchesOrigin(url: HttpUrl): Boolean =
        url.scheme == enteredUrl.scheme && url.host == enteredUrl.host && url.port == enteredUrl.port

    companion object {
        fun parse(text: String): PicHubConnection {
            val url = text.trim().toHttpUrlOrNull()
                ?: throw PicHubException("请输入完整的 HTTP 或 HTTPS 地址")
            if (url.username.isNotEmpty() || url.password.isNotEmpty() || url.fragment != null) {
                throw PicHubException("链接中不能包含账号、密码或片段")
            }
            val path = url.encodedPath.trimEnd('/')
            val endpoint = Regex("""/e/[a-z0-9-]+$""").find(path)
            val base = when {
                endpoint != null -> path.substring(0, endpoint.range.first)
                path.endsWith("/random") -> path.removeSuffix("/random")
                else -> path
            }
            val distribution = if (endpoint != null || path.endsWith("/random")) path else "$path/random"
            return PicHubConnection(
                enteredUrl = url,
                baseUrl = url.newBuilder().encodedPath("$base/").query(null).build(),
                distributionUrl = url.newBuilder().encodedPath(distribution).build()
            )
        }
    }
}

internal fun isPrivateAddress(address: InetAddress): Boolean {
    val bytes = address.address
    val carrierGrade = bytes.size == 4 && bytes[0].toInt() == 100 &&
        ((bytes[1].toInt() and 0xc0) == 0x40)
    val uniqueLocal = bytes.size == 16 && ((bytes[0].toInt() and 0xfe) == 0xfc)
    return address.isLoopbackAddress || address.isSiteLocalAddress || address.isLinkLocalAddress ||
        carrierGrade || uniqueLocal
}

internal class PicHubException(message: String) : Exception(message)
