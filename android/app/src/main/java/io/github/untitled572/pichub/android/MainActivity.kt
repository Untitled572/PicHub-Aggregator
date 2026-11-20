package io.github.untitled572.pichub.android

import android.content.res.Configuration
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.Image
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawing
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.selection.toggleable
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.FilledIconButton
import androidx.compose.material3.FilledIconToggleButton
import androidx.compose.material3.FilledTonalIconButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.IconButtonDefaults
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.dynamicDarkColorScheme
import androidx.compose.material3.dynamicLightColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextAlign
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
    BackHandler(enabled = model.showSettings, onBack = model::closeSettings)
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
        BoxWithConstraints(Modifier.fillMaxSize().padding(padding)) {
            val wide = maxWidth > maxHeight

            if (model.showSettings) {
                SettingsScreen(model, Modifier.fillMaxSize())
            } else {
                val showRail = wide && maxHeight >= 310.dp
                val labeledRefresh = maxWidth >= 330.dp
                val horizontalPadding = (maxWidth * 0.04f).coerceIn(12.dp, 28.dp)
                Column(
                    Modifier.fillMaxSize().padding(
                        start = horizontalPadding,
                        end = horizontalPadding,
                        bottom = if (wide) 6.dp else 10.dp
                    )
                ) {
                    Row(
                        Modifier.fillMaxWidth().heightIn(min = if (wide) 48.dp else 56.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.SpaceBetween
                    ) {
                        Text("PicHub", Modifier.padding(start = 4.dp), style = MaterialTheme.typography.titleLarge)
                        IconButton(onClick = model::openSettings) {
                            Icon(Icons.Filled.Settings, contentDescription = "设置")
                        }
                    }
                    if (showRail) {
                        Row(
                            Modifier.weight(1f).fillMaxWidth(),
                            horizontalArrangement = Arrangement.spacedBy(12.dp),
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            ImageStage(model, Modifier.weight(1f).fillMaxHeight())
                            Column(
                                horizontalAlignment = Alignment.CenterHorizontally,
                                verticalArrangement = Arrangement.spacedBy(6.dp)
                            ) {
                                ViewerActions(model, labeledRefresh = false)
                            }
                        }
                    } else {
                        ImageStage(model, Modifier.weight(1f).fillMaxWidth())
                        Row(
                            Modifier.align(Alignment.CenterHorizontally).widthIn(max = 480.dp)
                                .fillMaxWidth().padding(top = if (wide) 4.dp else 12.dp),
                            horizontalArrangement = Arrangement.SpaceEvenly,
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            ViewerActions(model, labeledRefresh = labeledRefresh)
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun ImageStage(model: ViewerModel, modifier: Modifier) {
    val shown = model.picture
    Surface(
        modifier = modifier,
        shape = RoundedCornerShape(24.dp),
        color = MaterialTheme.colorScheme.surfaceContainerLow
    ) {
        Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            when {
                shown != null -> {
                    val bitmap = remember(shown.picture.bitmap) { shown.picture.bitmap.asImageBitmap() }
                    Image(bitmap, contentDescription = "当前图片", Modifier.fillMaxSize(), contentScale = ContentScale.Fit)
                    if (model.loading) {
                        Surface(
                            Modifier.align(Alignment.TopEnd).padding(12.dp),
                            shape = CircleShape,
                            color = MaterialTheme.colorScheme.surfaceContainerHigh
                        ) {
                            CircularProgressIndicator(Modifier.padding(9.dp).size(20.dp), strokeWidth = 2.dp)
                        }
                    }
                }
                model.loading -> CircularProgressIndicator(Modifier.size(34.dp))
                else -> {
                    Column(
                        Modifier.padding(16.dp).widthIn(max = 320.dp).verticalScroll(rememberScrollState()),
                        horizontalAlignment = Alignment.CenterHorizontally,
                        verticalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        Text(
                            model.loadingError ?: "连接 PicHub 后开始看图",
                            textAlign = TextAlign.Center,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                        if (model.configured) {
                            OutlinedButton(onClick = model::refresh) { Text("重试") }
                        } else {
                            OutlinedButton(onClick = model::openSettings) { Text("连接服务器") }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun ViewerActions(model: ViewerModel, labeledRefresh: Boolean) {
    val shown = model.picture
    val refreshEnabled = model.configured && !model.loading
    if (labeledRefresh) {
        Button(
            onClick = model::refresh,
            enabled = refreshEnabled,
            modifier = Modifier.heightIn(min = 48.dp),
            contentPadding = PaddingValues(horizontal = 15.dp, vertical = 8.dp)
        ) {
            Icon(Icons.Filled.Refresh, contentDescription = null, Modifier.size(20.dp))
            Text("刷新", Modifier.padding(start = 6.dp))
        }
    } else {
        FilledIconButton(onClick = model::refresh, enabled = refreshEnabled) {
            Icon(Icons.Filled.Refresh, contentDescription = "刷新")
        }
    }

    FilledTonalIconButton(
        onClick = model::save,
        enabled = shown != null && !shown.saved && !model.saving
    ) {
        if (shown?.saved == true) {
            Icon(Icons.Filled.Check, contentDescription = "已保存")
        } else {
            Icon(painterResource(R.drawable.ic_download), contentDescription = "保存到相册")
        }
    }

    if (model.feedbackEnabled) {
        VoteButton(model, shown, Vote.Like, R.drawable.ic_thumb_up, "点赞")
        VoteButton(model, shown, Vote.Dislike, R.drawable.ic_thumb_down, "点踩")
    }
}

@Composable
private fun VoteButton(model: ViewerModel, shown: DisplayedPicture?, vote: Vote, icon: Int, label: String) {
    val selected = shown?.acceptedVote == vote
    FilledIconToggleButton(
        checked = selected,
        onCheckedChange = { model.vote(vote) },
        enabled = shown?.picture?.id != null && model.feedbackEnabled && !model.voting &&
            shown.acceptedVote == null && (shown.attemptedVote == null || shown.attemptedVote == vote),
        modifier = Modifier.semantics { if (selected) stateDescription = "已反馈" },
        colors = IconButtonDefaults.filledIconToggleButtonColors(
            containerColor = MaterialTheme.colorScheme.surfaceContainerHighest,
            contentColor = MaterialTheme.colorScheme.onSurfaceVariant,
            disabledContainerColor = if (selected) MaterialTheme.colorScheme.primaryContainer
                else MaterialTheme.colorScheme.onSurface.copy(alpha = 0.10f),
            disabledContentColor = if (selected) MaterialTheme.colorScheme.onPrimaryContainer
                else MaterialTheme.colorScheme.onSurface.copy(alpha = 0.38f)
        )
    ) {
        Icon(painterResource(icon), contentDescription = label)
    }
}

@Composable
private fun SettingsScreen(model: ViewerModel, modifier: Modifier) {
    Column(
        modifier.padding(horizontal = 12.dp),
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Row(
            Modifier.widthIn(max = 560.dp).fillMaxWidth().height(52.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            IconButton(onClick = model::closeSettings) {
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
            }
            if (model.draftAddress.trim().startsWith("http://", ignoreCase = true)) {
                Text(
                    "局域网 HTTP 连接使用明文传输。",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
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
                Button(onClick = model::applySettings, enabled = model.draftAddress.isNotBlank()) {
                    Text("保存")
                }
            }
        }
    }
}
