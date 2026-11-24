package io.github.untitled572.pichub.android

import android.content.res.Configuration
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawing
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.selection.toggleable
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.Checkbox
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.dynamicDarkColorScheme
import androidx.compose.material3.dynamicLightColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            PicHubTheme {
                PicHubScreen(viewModel())
            }
        }
    }
}

@Composable
private fun PicHubTheme(content: @Composable () -> Unit) {
    val dark = isSystemInDarkTheme()
    val context = LocalContext.current
    val colors = if (Build.VERSION.SDK_INT >= 31) {
        if (dark) dynamicDarkColorScheme(context) else dynamicLightColorScheme(context)
    } else if (dark) {
        darkColorScheme(
            primary = Color(0xFFA9DCCC), onPrimary = Color(0xFF0B382F),
            primaryContainer = Color(0xFF205044), onPrimaryContainer = Color(0xFFC5F8E8),
            surface = Color(0xFF121715), background = Color(0xFF121715)
        )
    } else {
        lightColorScheme(
            primary = Color(0xFF235C52), onPrimary = Color.White,
            primaryContainer = Color(0xFFB1EEE0), onPrimaryContainer = Color(0xFF0B382F),
            surface = Color(0xFFF7FAF8), background = Color(0xFFF7FAF8)
        )
    }
    MaterialTheme(colorScheme = colors, content = content)
}

@Composable
private fun PicHubScreen(model: ViewerModel) {
    val snackbar = remember { SnackbarHostState() }
    val notice = model.notice
    LaunchedEffect(notice) {
        if (notice != null) {
            snackbar.showSnackbar(notice)
            model.dismissNotice(notice)
        }
    }
    BackHandler(enabled = model.showSettings, onBack = model::applySettings)
    val direction = if (LocalConfiguration.current.orientation == Configuration.ORIENTATION_LANDSCAPE) {
        PictureDirection.Horizontal
    } else {
        PictureDirection.Vertical
    }
    LaunchedEffect(direction) { model.setDirection(direction) }

    Scaffold(
        contentWindowInsets = WindowInsets.safeDrawing,
        snackbarHost = { SnackbarHost(snackbar) }
    ) { padding ->
        if (model.showSettings) {
            SettingsScreen(model, Modifier.fillMaxSize().padding(padding))
        } else {
            ViewerScreen(model)
        }
    }
}

@Composable
private fun SettingsScreen(model: ViewerModel, modifier: Modifier) {
    var choosingTags by rememberSaveable { mutableStateOf(false) }
    if (choosingTags) TagPicker(model) { choosingTags = false }
    val directoryPicker = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocumentTree()) { tree ->
        if (tree != null) model.updateSaveTree(tree)
    }
    Column(
        modifier.padding(horizontal = 12.dp),
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Row(
            Modifier.widthIn(max = 560.dp).fillMaxWidth().height(52.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            IconButton(onClick = model::applySettings) {
                Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "返回")
            }
            Text("设置", style = MaterialTheme.typography.titleLarge)
        }
        Column(
            Modifier.widthIn(max = 560.dp).fillMaxWidth().verticalScroll(rememberScrollState())
                .padding(horizontal = 12.dp, vertical = 12.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp)
        ) {
            OutlinedTextField(
                value = model.draftAddress,
                onValueChange = model::updateAddress,
                modifier = Modifier.fillMaxWidth(),
                label = { Text("PicHub 链接") },
                singleLine = true,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri),
                supportingText = { Text("可输入服务地址或完整分发链接") }
            )
            ListItem(
                modifier = Modifier.fillMaxWidth().toggleable(
                    value = model.draftFeedbackEnabled,
                    role = Role.Switch,
                    onValueChange = model::updateFeedbackEnabled
                ),
                headlineContent = { Text("启用点赞和点踩") },
                supportingContent = { Text("关闭时只看图和保存，无需 App 密钥") },
                trailingContent = { Switch(checked = model.draftFeedbackEnabled, onCheckedChange = null) }
            )
            if (model.draftFeedbackEnabled) {
                OutlinedTextField(
                    value = model.draftKey,
                    onValueChange = model::updateKey,
                    modifier = Modifier.fillMaxWidth(),
                    label = { Text("App 密钥") },
                    singleLine = true,
                    visualTransformation = PasswordVisualTransformation(),
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password),
                    supportingText = { Text("在控制台的「设置 → 安全鉴权」中生成") }
                )
                if (model.canChooseTags) {
                    ListItem(
                        headlineContent = { Text("App 取图范围") },
                        supportingContent = { Text(model.draftScopeLabel) },
                        trailingContent = { TextButton(onClick = { choosingTags = true }) { Text("选择 Tag") } }
                    )
                }
            }
            if (model.draftAddress.trim().startsWith("http://", ignoreCase = true)) {
                Text(
                    "局域网 HTTP 连接使用明文传输。",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text("保存路径", style = MaterialTheme.typography.titleSmall)
                Text(
                    saveDirectoryLabel(model.draftOptions.saveTree),
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    OutlinedButton(onClick = { directoryPicker.launch(model.draftOptions.saveTree) }) {
                        Text("选择目录")
                    }
                    if (model.draftOptions.saveTree != null) {
                        TextButton(onClick = { model.updateSaveTree(null) }) { Text("恢复默认") }
                    }
                }
            }
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text("图片显示", style = MaterialTheme.typography.titleSmall)
                SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth()) {
                    SegmentedButton(
                        selected = !model.draftOptions.fillScreen,
                        onClick = { model.updateFillScreen(false) },
                        shape = SegmentedButtonDefaults.itemShape(index = 0, count = 2)
                    ) { Text("完整显示") }
                    SegmentedButton(
                        selected = model.draftOptions.fillScreen,
                        onClick = { model.updateFillScreen(true) },
                        shape = SegmentedButtonDefaults.itemShape(index = 1, count = 2)
                    ) { Text("铺满屏幕") }
                }
            }
            ListItem(
                modifier = Modifier.fillMaxWidth().toggleable(
                    value = model.draftOptions.showDetails,
                    role = Role.Checkbox,
                    onValueChange = model::updateShowDetails
                ),
                headlineContent = { Text("显示详情") },
                trailingContent = { Checkbox(checked = model.draftOptions.showDetails, onCheckedChange = null) }
            )
            if (model.connectionResult.isNotEmpty()) {
                Surface(
                    shape = RoundedCornerShape(16.dp),
                    color = MaterialTheme.colorScheme.surfaceContainerLow
                ) {
                    Column(Modifier.fillMaxWidth().padding(14.dp), verticalArrangement = Arrangement.spacedBy(5.dp)) {
                        model.connectionResult.forEach { Text(it, style = MaterialTheme.typography.bodyMedium) }
                    }
                }
            }
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp, Alignment.End)) {
                OutlinedButton(onClick = model::testConnection, enabled = !model.testing && model.draftAddress.isNotBlank()) {
                    if (model.testing) {
                        CircularProgressIndicator(Modifier.size(16.dp), strokeWidth = 2.dp)
                    } else {
                        Text("测试连接")
                    }
                }
            }
        }
    }
}
