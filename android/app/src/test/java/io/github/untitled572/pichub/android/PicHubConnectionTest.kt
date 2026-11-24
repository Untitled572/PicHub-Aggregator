package io.github.untitled572.pichub.android

import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test
import java.net.InetAddress

class PicHubConnectionTest {
    @Test
    fun standardAddressUsesRandomDistribution() {
        val server = PicHubConnection.parse(" https://pichub.example.org/ ")
        assertEquals(
            "https://pichub.example.org/random?format=json&orientation=vertical",
            server.nextImage(PictureDirection.Vertical).toString()
        )
        assertEquals("https://pichub.example.org/api/app/feedback", server.api("feedback").toString())
    }

    @Test
    fun endpointPreservesPrefixAndUserFilters() {
        val connection = PicHubConnection.parse(
            "http://pichub.local:5721/gallery/e/wallpaper?category=anime&format=raw&orientation=vertical"
        )
        assertEquals(
            "http://pichub.local:5721/gallery/e/wallpaper?category=anime&format=json",
            connection.nextImage(connection.pictureDirection(PictureDirection.Horizontal)).toString()
        )
        assertEquals(
            "http://pichub.local:5721/gallery/images/some-file",
            connection.imageUrl("/images/some-file").toString()
        )
        assertEquals(
            "http://pichub.local:5721/gallery/api/app/auth/check",
            connection.api("auth", "check").toString()
        )
        assertTrue(connection.matchesOrigin("http://pichub.local:5721/images/test".toHttpUrl()))
        assertFalse(connection.matchesOrigin("http://pichub.local:5722/images/test".toHttpUrl()))
        assertFalse(connection.matchesOrigin("https://pichub.local:5721/images/test".toHttpUrl()))
    }

    @Test
    fun selectingTagsPreservesNamedEndpointAndOtherFilters() {
        val named = PicHubConnection.parse(
            "https://example.org/pic/e/other?category=horizontal&quality=high"
        )
        val selected = PicHubConnection.parse(named.withTags(listOf("vertical", "adaptive")))
        val request = selected.nextImage(PictureDirection.Vertical)
        assertEquals("/pic/e/other", request.encodedPath)
        assertEquals("high", request.queryParameter("quality"))
        assertEquals("vertical,adaptive", request.queryParameter("category"))
        assertEquals(listOf("vertical", "adaptive"), selected.tags)

        val inherited = PicHubConnection.parse(selected.withTags(emptyList()))
        assertTrue(inherited.tags.isEmpty())
        assertEquals("/pic/e/other", inherited.nextImage(PictureDirection.Vertical).encodedPath)
        assertEquals("high", inherited.nextImage(PictureDirection.Vertical).queryParameter("quality"))
    }

    @Test
    fun selectingTagsOnServerAddressKeepsTheMasterDistribution() {
        val server = PicHubConnection.parse("https://example.org/pic")
        val selected = PicHubConnection.parse(server.withTags(listOf("vertical")))
        val request = selected.nextImage(PictureDirection.Vertical)
        assertEquals("/pic/random", request.encodedPath)
        assertEquals("vertical", request.queryParameter("category"))
    }

    @Test
    fun aSingleDirectionTagOverridesTheScreenForThePictureRequest() {
        val server = PicHubConnection.parse("https://example.org/pic/e/other")
        fun requested(tags: List<String>, screen: PictureDirection): String? {
            val selected = PicHubConnection.parse(server.withTags(tags))
            return selected.nextImage(selected.pictureDirection(screen)).queryParameter("orientation")
        }
        assertEquals("horizontal", requested(listOf("horizontal"), PictureDirection.Vertical))
        assertEquals("vertical", requested(listOf("vertical", "anime"), PictureDirection.Horizontal))
        assertEquals("horizontal", requested(listOf("adaptive"), PictureDirection.Horizontal))
    }

    @Test
    fun mixedDirectionsAndCustomTagsDoNotForceTheScreenDirection() {
        val server = PicHubConnection.parse("https://example.org/e/wallpaper?orientation=vertical")
        val tags = listOf(listOf("horizontal", "vertical"), listOf("anime"), listOf("square"))
        for (tagSet in tags) {
            val configured = PicHubConnection.parse(server.withTags(tagSet))
            for (screen in PictureDirection.entries) {
                val direction = configured.pictureDirection(screen)
                assertEquals(null, direction)
                val request = configured.nextImage(direction)
                assertEquals(null, request.queryParameter("orientation"))
                assertEquals(tagSet.joinToString(","), request.queryParameter("category"))
                assertEquals("/e/wallpaper", request.encodedPath)
            }
        }
    }

    @Test
    fun retryAfterAcceptsSecondsAndHttpDate() {
        val now = java.time.Instant.parse("2026-11-20T08:00:00Z").toEpochMilli()
        assertEquals(3_000L, retryAfterMillis("3", now))
        assertEquals(8_000L, retryAfterMillis("Fri, 20 Nov 2026 08:00:08 GMT", now))
        assertEquals(0L, retryAfterMillis("Fri, 20 Nov 2026 07:59:58 GMT", now))
        assertEquals(null, retryAfterMillis("invalid", now))
    }

    @Test
    fun malformedLinksAreRejected() {
        assertThrows(PicHubException::class.java) { PicHubConnection.parse("pichub.local") }
        assertThrows(PicHubException::class.java) { PicHubConnection.parse("https://u:p@example.org") }
        assertThrows(PicHubException::class.java) { PicHubConnection.parse("https://example.org/#home") }
    }

    @Test
    fun lanTransportRejectsPublicAddresses() {
        for (ip in listOf("127.0.0.1", "10.0.2.2", "192.168.1.5", "172.16.2.1", "100.64.1.4", "fd12::7")) {
            assertTrue(ip, isPrivateAddress(InetAddress.getByName(ip)))
        }
        for (ip in listOf("192.169.0.1", "8.8.8.8", "100.128.1.4", "2001:4860:4860::8888")) {
            assertFalse(ip, isPrivateAddress(InetAddress.getByName(ip)))
        }
    }
}
