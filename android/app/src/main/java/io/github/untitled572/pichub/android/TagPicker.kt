package io.github.untitled572.pichub.android

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.toggleable
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Checkbox
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ListItem
import androidx.compose.material3.ListItemDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.CancellationException

@Composable
internal fun TagPicker(model: ViewerModel, dismiss: () -> Unit) {
    var tags by remember { mutableStateOf<List<AppTag>?>(null) }
    val initialTags = remember { model.draftTags.toSet() }
    var selected by remember { mutableStateOf(initialTags) }
    var error by remember { mutableStateOf<String?>(null) }
    var attempt by remember { mutableStateOf(0) }
    LaunchedEffect(attempt) {
        error = null
        try {
            tags = model.listDraftTags()
        } catch (cause: CancellationException) {
            throw cause
        } catch (cause: Exception) {
            error = cause.userMessage("无法读取标签，请检查服务地址和 App 密钥")
        }
    }

    val saveAndReturn: () -> Unit = {
        val ids = tags.orEmpty().map { it.id }
        if (ids.isNotEmpty() && selected != initialTags) {
            val active = ids.filter { it in selected }
            model.selectDraftTags(if (active.size == ids.size) active + "__uncategorized__" else active)
        }
        dismiss()
    }
    AlertDialog(
        onDismissRequest = saveAndReturn,
        title = { Text("App 取图范围", style = MaterialTheme.typography.titleLarge) },
        shape = RoundedCornerShape(16.dp),
        containerColor = MaterialTheme.colorScheme.surfaceContainerLow,
        text = {
            val message = error
            val choices = tags
            when {
                message != null -> Text(message)
                choices == null -> CircularProgressIndicator()
                else -> Column(
                    Modifier.heightIn(max = 330.dp).verticalScroll(rememberScrollState()),
                    verticalArrangement = Arrangement.spacedBy(2.dp)
                ) {
                    Text("只影响此 App 的取图。未选标签时，沿用当前接口的默认范围。", style = MaterialTheme.typography.bodySmall)
                    val all = choices.isNotEmpty() && choices.all { it.id in selected }
                    TagChoice("全部标签", all) {
                        selected = if (it) choices.mapTo(mutableSetOf()) { tag -> tag.id } else emptySet()
                    }
                    choices.forEach { tag ->
                        TagChoice("#${tag.name} (${tag.id})", tag.id in selected) {
                            selected = if (it) selected + tag.id else selected - tag.id
                        }
                    }
                }
            }
        },
        confirmButton = {
            if (error != null) {
                TextButton(onClick = { attempt += 1 }) { Text("重试") }
            }
        }
    )
}

@Composable
private fun TagChoice(label: String, checked: Boolean, toggle: (Boolean) -> Unit) {
    ListItem(
        modifier = Modifier.fillMaxWidth().toggleable(value = checked, role = Role.Checkbox, onValueChange = toggle),
        colors = ListItemDefaults.colors(containerColor = Color.Transparent),
        headlineContent = { Text(label) },
        leadingContent = { Checkbox(checked = checked, onCheckedChange = null) }
    )
}
