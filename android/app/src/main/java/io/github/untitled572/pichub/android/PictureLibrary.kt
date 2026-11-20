package io.github.untitled572.pichub.android

import android.content.ContentResolver
import android.content.ContentValues
import android.content.Intent
import android.net.Uri
import android.os.Environment
import android.provider.DocumentsContract
import android.provider.MediaStore
import android.webkit.MimeTypeMap
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ensureActive
import kotlinx.coroutines.withContext
import java.io.IOException
import kotlin.coroutines.coroutineContext

private val galleryDirectory = Environment.DIRECTORY_PICTURES + "/PicHub"

internal const val folderPermissionFlags = Intent.FLAG_GRANT_READ_URI_PERMISSION or
    Intent.FLAG_GRANT_WRITE_URI_PERMISSION

internal fun saveDirectoryLabel(tree: Uri?): String {
    if (tree == null) return galleryDirectory
    if (tree.authority == "com.android.externalstorage.documents") {
        val documentId = DocumentsContract.getTreeDocumentId(tree)
        val path = documentId.substringAfter(':', "")
        if (path.isNotBlank()) return path
    }
    return "所选目录"
}

internal suspend fun savePicture(resolver: ContentResolver, picture: Picture, tree: Uri?) =
    withContext(Dispatchers.IO) {
        val extension = when (picture.mimeType) {
            "image/jpeg" -> "jpg"
            else -> MimeTypeMap.getSingleton().getExtensionFromMimeType(picture.mimeType)
                ?: throw PicHubException("无法识别图片文件格式")
        }
        val timestamp = System.currentTimeMillis()
        val filename = "PicHub-$timestamp.$extension"
        val destination = if (tree == null) {
            val values = ContentValues().apply {
                put(MediaStore.Images.Media.DISPLAY_NAME, filename)
                put(MediaStore.Images.Media.MIME_TYPE, picture.mimeType)
                put(MediaStore.Images.Media.RELATIVE_PATH, galleryDirectory)
                put(MediaStore.Images.Media.IS_PENDING, 1)
            }
            resolver.insert(MediaStore.Images.Media.EXTERNAL_CONTENT_URI, values)
        } else {
            createInDirectory(resolver, tree, picture.mimeType, filename)
        } ?: throw IOException("Cannot create image file")

        var saved = false
        try {
            val output = resolver.openOutputStream(destination, "w")
                ?: throw IOException("Cannot open image file")
            output.use { stream ->
                picture.original.inputStream().use { input ->
                    val buffer = ByteArray(32 * 1024)
                    while (true) {
                        coroutineContext.ensureActive()
                        val count = input.read(buffer)
                        if (count < 0) break
                        stream.write(buffer, 0, count)
                    }
                }
            }
            coroutineContext.ensureActive()
            if (tree == null) {
                val published = ContentValues().apply { put(MediaStore.Images.Media.IS_PENDING, 0) }
                if (resolver.update(destination, published, null, null) != 1) {
                    throw IOException("Cannot publish gallery image")
                }
            }
            saved = true
        } finally {
            if (!saved) {
                runCatching {
                    if (tree == null) resolver.delete(destination, null, null)
                    else DocumentsContract.deleteDocument(resolver, destination)
                }
            }
        }
    }

private fun createInDirectory(resolver: ContentResolver, tree: Uri, mimeType: String, filename: String): Uri? {
    val folder = DocumentsContract.buildDocumentUriUsingTree(tree, DocumentsContract.getTreeDocumentId(tree))
    return DocumentsContract.createDocument(resolver, folder, mimeType, filename)
}
