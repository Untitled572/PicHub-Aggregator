package io.github.untitled572.pichub.android

import android.content.ContentResolver
import android.content.ContentValues
import android.os.Environment
import android.provider.MediaStore
import android.webkit.MimeTypeMap
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.IOException

internal suspend fun saveToGallery(resolver: ContentResolver, picture: Picture) =
    withContext(Dispatchers.IO) {
        val extension = when (picture.mimeType) {
            "image/jpeg" -> "jpg"
            else -> MimeTypeMap.getSingleton().getExtensionFromMimeType(picture.mimeType)
                ?: throw PicHubException("相册不支持这张图的文件格式")
        }
        val timestamp = System.currentTimeMillis()
        val values = ContentValues().apply {
            put(MediaStore.Images.Media.DISPLAY_NAME, "PicHub-$timestamp.$extension")
            put(MediaStore.Images.Media.MIME_TYPE, picture.mimeType)
            put(MediaStore.Images.Media.RELATIVE_PATH, Environment.DIRECTORY_PICTURES + "/PicHub")
            put(MediaStore.Images.Media.IS_PENDING, 1)
        }
        val uri = resolver.insert(MediaStore.Images.Media.EXTERNAL_CONTENT_URI, values)
            ?: throw IOException("MediaStore insert failed")
        var saved = false
        try {
            val output = resolver.openOutputStream(uri) ?: throw IOException("Cannot open media file")
            output.use { stream ->
                picture.original.inputStream().use { it.copyTo(stream) }
            }
            val published = ContentValues().apply { put(MediaStore.Images.Media.IS_PENDING, 0) }
            if (resolver.update(uri, published, null, null) != 1) throw IOException("Cannot publish media file")
            saved = true
        } finally {
            if (!saved) resolver.delete(uri, null, null)
        }
    }
