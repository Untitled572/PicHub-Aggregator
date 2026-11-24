package io.github.untitled572.pichub.android

import android.text.format.Formatter
import androidx.activity.compose.BackHandler
import androidx.activity.compose.LocalActivity
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.spring
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectDragGestures
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.WindowInsetsSides
import androidx.compose.foundation.layout.asPaddingValues
import androidx.compose.foundation.layout.displayCutout
import androidx.compose.foundation.layout.navigationBarsIgnoringVisibility
import androidx.compose.foundation.layout.statusBarsIgnoringVisibility
import androidx.compose.foundation.layout.union
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.only
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.FilledIconButton
import androidx.compose.material3.FilledIconToggleButton
import androidx.compose.material3.FilledTonalIconButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.IconButtonDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clipToBounds
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.input.pointer.util.VelocityTracker
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.CustomAccessibilityAction
import androidx.compose.ui.semantics.customActions
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.onClick
import androidx.compose.ui.semantics.onLongClick
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.WindowInsetsControllerCompat
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import java.util.Locale
import kotlin.math.abs

private val viewerColors = darkColorScheme(
    primary = Color(0xFFA9C5DA),
    onPrimary = Color(0xFF203A4D),
    primaryContainer = Color(0xFF4A6B82),
    onPrimaryContainer = Color(0xFFEDF3F8),
    secondaryContainer = Color(0xFF34384D),
    onSecondaryContainer = Color(0xFFE2E3FB),
    surface = Color(0xFF242733),
    onSurface = Color(0xFFEEEFF8),
    onSurfaceVariant = Color(0xFFC1C5D7),
    surfaceContainerHighest = Color(0xFF363A4C)
)

@OptIn(ExperimentalLayoutApi::class)
@Composable
internal fun ViewerScreen(model: ViewerModel) {
    var expanded by rememberSaveable { mutableStateOf(true) }
    var dragProgress by remember { mutableStateOf<Float?>(null) }
    val picture = model.picture
    val toolsVisible = expanded || picture == null
    val animation = remember { Animatable(if (expanded) 1f else 0f).apply { updateBounds(0f, 1f) } }
    val scope = rememberCoroutineScope()
    var settling by remember { mutableStateOf<Job?>(null) }

    fun settle(open: Boolean, velocity: Float = 0f) {
        expanded = open
        settling?.cancel()
        settling = scope.launch {
            dragProgress?.let { animation.snapTo(it) }
            dragProgress = null
            animation.animateTo(
                if (open) 1f else 0f,
                animationSpec = spring(dampingRatio = 1f, stiffness = 500f),
                initialVelocity = velocity
            )
        }
    }

    val showTools = { settle(true) }
    val hideTools = { settle(false) }
    val reveal = { if (model.picture == null) 1f else dragProgress ?: animation.value }
    val settledOpen by remember { derivedStateOf { reveal() >= 0.98f } }
    val panelVisible by remember { derivedStateOf { reveal() > 0.08f } }
    BackHandler(enabled = !toolsVisible, onBack = showTools)

    MaterialTheme(colorScheme = viewerColors) {
        BoxWithConstraints(
            Modifier.fillMaxSize()
                .background(Color(0xFF14161D))
                .semantics {
                    customActions = buildList {
                        if (picture != null) {
                            add(CustomAccessibilityAction(if (toolsVisible) "收起工具栏" else "显示工具栏") {
                                if (toolsVisible) hideTools() else showTools()
                                true
                            })
                        }
                        if (model.configured && !model.loading) add(CustomAccessibilityAction("下一张") {
                            model.nextPicture()
                            true
                        })
                        if (model.canShowPrevious) add(CustomAccessibilityAction("上一张") {
                            model.previousPicture()
                            true
                        })
                    }
                }
        ) {
            val landscape = maxWidth > maxHeight
            val scale = when {
                !model.viewerOptions.fillScreen -> ContentScale.Fit
                landscape -> ContentScale.FillHeight
                else -> ContentScale.Crop
            }
            val navigationInsets = WindowInsets.navigationBarsIgnoringVisibility.union(WindowInsets.displayCutout)
            val stableInsets = if (landscape) navigationInsets
            else navigationInsets.union(WindowInsets.statusBarsIgnoringVisibility)
            val panelInsets = navigationInsets.only(WindowInsetsSides.Bottom + WindowInsetsSides.Horizontal)
            ViewerSystemBars(visible = toolsVisible && settledOpen, landscape = landscape)

            Box(
                Modifier.fillMaxSize().viewerSwipes(
                    landscape = landscape,
                    canDragPanel = picture != null,
                    currentReveal = reveal,
                    dragPanel = {
                        settling?.cancel()
                        dragProgress = it
                    },
                    settlePanel = { open, velocity -> settle(open ?: expanded, velocity) },
                    next = model::nextPicture,
                    previous = model::previousPicture
                )
            ) {
                ViewerLayout(
                    landscape = landscape,
                    hasDetails = model.viewerOptions.showDetails && picture != null,
                    stableInsets = stableInsets,
                    reveal = reveal,
                    image = { ImageStage(model, Modifier.fillMaxSize(), scale, toolsVisible, showTools) },
                    panel = { ViewerPanel(model, landscape, panelInsets.asPaddingValues()) },
                    modifier = Modifier.fillMaxSize()
                )
                if (model.loading && !panelVisible) {
                    LoadingRing(
                        Modifier.align(Alignment.BottomStart)
                            .windowInsetsPadding(WindowInsets.displayCutout.only(WindowInsetsSides.Start + WindowInsetsSides.Bottom))
                            .padding(18.dp).size(25.dp)
                    )
                }
            }
        }
    }
}

@Composable
private fun ViewerSystemBars(visible: Boolean, landscape: Boolean) {
    val window = LocalActivity.current?.window ?: return
    val controller = remember(window) { WindowCompat.getInsetsController(window, window.decorView) }
    DisposableEffect(controller) {
        val previousBehavior = controller.systemBarsBehavior
        val previousStatus = controller.isAppearanceLightStatusBars
        val previousNav = controller.isAppearanceLightNavigationBars
        controller.systemBarsBehavior = WindowInsetsControllerCompat.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE
        controller.isAppearanceLightStatusBars = false
        controller.isAppearanceLightNavigationBars = false
        onDispose {
            controller.show(WindowInsetsCompat.Type.systemBars())
            controller.systemBarsBehavior = previousBehavior
            controller.isAppearanceLightStatusBars = previousStatus
            controller.isAppearanceLightNavigationBars = previousNav
        }
    }
    LaunchedEffect(controller, visible, landscape) {
        if (visible) controller.show(WindowInsetsCompat.Type.navigationBars())
        else controller.hide(WindowInsetsCompat.Type.navigationBars())
        if (visible && !landscape) controller.show(WindowInsetsCompat.Type.statusBars())
        else controller.hide(WindowInsetsCompat.Type.statusBars())
    }
}

@Composable
private fun ImageStage(
    model: ViewerModel,
    modifier: Modifier,
    contentScale: ContentScale,
    toolsVisible: Boolean,
    showTools: () -> Unit
) {
    val shown = model.picture
    val saveEnabled = shown != null && !shown.saved && !model.saving
    val haptic = LocalHapticFeedback.current
    Box(modifier.clipToBounds(), contentAlignment = Alignment.Center) {
        when {
            shown != null -> {
                val bitmap = remember(shown.picture.bitmap) { shown.picture.bitmap.asImageBitmap() }
                Image(
                    bitmap = bitmap,
                    contentDescription = "当前图片",
                    modifier = Modifier.fillMaxSize()
                        .pointerInput(shown.requestId, saveEnabled, toolsVisible, showTools) {
                            detectTapGestures(
                                onLongPress = if (saveEnabled) {
                                    {
                                        haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                                        model.save()
                                    }
                                } else null,
                                onTap = if (toolsVisible) null else { { showTools() } }
                            )
                        }
                        .semantics {
                            if (saveEnabled) onLongClick(label = "保存原图") { model.save(); true }
                            if (!toolsVisible) onClick(label = "显示工具栏") { showTools(); true }
                        },
                    contentScale = contentScale
                )
            }
            model.loading -> Unit
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

@Composable
private fun ViewerPanel(model: ViewerModel, landscape: Boolean, panelInsets: PaddingValues) {
    val details = model.picture?.picture?.takeIf { model.viewerOptions.showDetails }
    Surface(color = MaterialTheme.colorScheme.surface, contentColor = MaterialTheme.colorScheme.onSurface) {
        if (landscape) {
            BoxWithConstraints(Modifier.fillMaxSize()) {
                val floor = (maxHeight * 0.08f).coerceAtMost(24.dp)
                Row(Modifier.fillMaxSize()) {
                    if (details != null) {
                        PictureInformation(
                            details, Modifier.weight(1f).align(Alignment.Bottom).padding(bottom = floor),
                            landscape = true
                        )
                    }
                    Column(
                        Modifier.width(52.dp)
                            .align(if (details == null) Alignment.CenterVertically else Alignment.Bottom)
                            .padding(bottom = if (details == null) 0.dp else floor),
                        horizontalAlignment = Alignment.CenterHorizontally
                    ) {
                        ViewerActions(model)
                    }
                }
            }
        } else {
            Column(
                Modifier.padding(panelInsets),
                horizontalAlignment = Alignment.CenterHorizontally
            ) {
                if (details != null) {
                    PictureInformation(details, Modifier.widthIn(max = 680.dp).fillMaxWidth(), landscape = false)
                }
                Row(
                    Modifier.widthIn(max = 480.dp).fillMaxWidth().heightIn(min = 52.dp),
                    horizontalArrangement = Arrangement.SpaceEvenly,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    ViewerActions(model)
                }
            }
        }
    }
}

@Composable
private fun PictureInformation(picture: Picture, modifier: Modifier, landscape: Boolean) {
    val details = picture.details
    val context = LocalContext.current
    val size = Formatter.formatShortFileSize(context, details.bytes)
    val format = picture.mimeType.substringAfter('/').uppercase(Locale.ROOT)
    val dimensions = "${details.width} × ${details.height}"
    val categories = details.categories.filterNot { it == "__uncategorized__" }
    val source = details.source?.substringBefore(" › ") ?: "未提供"
    val branch = details.source?.substringAfter(" › ", "")?.takeIf { it.isNotBlank() } ?: "主地址"
    val tags = categories.joinToString(" · ")
    Column(
        modifier.padding(start = if (landscape) 9.dp else 14.dp, end = if (landscape) 2.dp else 10.dp, top = 8.dp, bottom = 5.dp),
        horizontalAlignment = Alignment.Start,
        verticalArrangement = Arrangement.spacedBy(3.dp)
    ) {
        Text(
            "图源  $source",
            style = MaterialTheme.typography.labelLarge,
            maxLines = if (landscape) 2 else 1,
            overflow = TextOverflow.Ellipsis
        )
        val response = "HTTP\u00A0${details.responseCode}"
        val duration = "${details.elapsedMillis}\u00A0ms"
        if (landscape) {
            DetailLine(dimensions, 1)
            DetailLine("$format · $size", 1)
            DetailLine(details.requestPath, 1)
            DetailLine("$response · $duration", 1)
            DetailLine("分支: $branch", 1)
            DetailLine("标签: ${tags.ifEmpty { "无" }}", 1)
        } else {
            DetailLine("$dimensions · $format · $size", 1)
            DetailLine("${details.requestPath} · $response · $duration", 1)
            val tagSuffix = if (tags.isEmpty()) "" else " · 标签 $tags"
            DetailLine("分支: $branch$tagSuffix", 1)
        }
    }
}

@Composable
private fun DetailLine(value: String, maxLines: Int) {
    Text(
        value,
        style = MaterialTheme.typography.bodySmall,
        color = MaterialTheme.colorScheme.onSurfaceVariant,
        maxLines = maxLines,
        overflow = TextOverflow.Ellipsis
    )
}

@Composable
private fun ViewerActions(model: ViewerModel) {
    val shown = model.picture
    Box(Modifier.size(48.dp), contentAlignment = Alignment.Center) {
        FilledIconButton(onClick = model::nextPicture, enabled = model.configured && !model.loading) {
            Icon(Icons.Filled.Refresh, contentDescription = "刷新")
        }
        if (model.loading) LoadingRing(Modifier.size(47.dp))
    }
    FilledTonalIconButton(onClick = model::save, enabled = shown != null && !shown.saved && !model.saving) {
        if (shown?.saved == true) {
            Icon(Icons.Filled.Check, contentDescription = "已保存")
        } else {
            Icon(painterResource(R.drawable.ic_download), contentDescription = "保存图片")
        }
    }
    if (model.feedbackEnabled) {
        VoteButton(model, shown, Vote.Like, R.drawable.ic_thumb_up, "点赞")
        VoteButton(model, shown, Vote.Dislike, R.drawable.ic_thumb_down, "点踩")
    }
    IconButton(onClick = model::openSettings) {
        Icon(Icons.Filled.Settings, contentDescription = "设置")
    }
}

@Composable
private fun VoteButton(model: ViewerModel, shown: DisplayedPicture?, vote: Vote, icon: Int, label: String) {
    val selected = shown?.acceptedVote == vote
    FilledIconToggleButton(
        checked = selected,
        onCheckedChange = { model.vote(vote) },
        enabled = model.canVote(vote),
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
        Icon(painterResource(icon), contentDescription = if (selected && shown?.picture?.distributionRef != null) "取消$label" else label)
    }
}

@Composable
private fun LoadingRing(modifier: Modifier) {
    CircularProgressIndicator(modifier.semantics { contentDescription = "正在取图" }, strokeWidth = 2.dp)
}

@Composable
private fun Modifier.viewerSwipes(
    landscape: Boolean,
    canDragPanel: Boolean,
    currentReveal: () -> Float,
    dragPanel: (Float) -> Unit,
    settlePanel: (Boolean?, Float) -> Unit,
    next: () -> Unit,
    previous: () -> Unit
): Modifier {
    val current by rememberUpdatedState(currentReveal)
    val dragging by rememberUpdatedState(dragPanel)
    val finish by rememberUpdatedState(settlePanel)
    val goNext by rememberUpdatedState(next)
    val goBack by rememberUpdatedState(previous)
    return pointerInput(landscape, canDragPanel) {
        val threshold = 42.dp.toPx()
        val travel = if (landscape) 220.dp.toPx() else 150.dp.toPx()
        val direction = if (landscape) 1 else -1
        var distance = Offset.Zero
        var start = 0f
        var vertical: Boolean? = null
        val velocity = VelocityTracker()
        detectDragGestures(
            onDragStart = {
                velocity.resetTracking()
                distance = Offset.Zero
                vertical = null
                start = current()
            },
            onDrag = { change, amount ->
                distance += amount
                if (vertical == null && distance != Offset.Zero) vertical = abs(distance.y) > abs(distance.x)
                if (vertical == true && canDragPanel) {
                    velocity.addPosition(change.uptimeMillis, change.position)
                    dragging((start + direction * distance.y / travel).coerceIn(0f, 1f))
                }
            },
            onDragCancel = { if (vertical == true && canDragPanel) finish(null, 0f) },
            onDragEnd = {
                if (vertical == true && canDragPanel) {
                    val opening = if (abs(distance.y) > threshold) direction * distance.y > 0f else null
                    val speed = (direction * velocity.calculateVelocity().y / travel).coerceIn(-5f, 5f)
                    finish(opening, speed)
                } else if (vertical == false) {
                    if (distance.x < -threshold) goNext()
                    else if (distance.x > threshold) goBack()
                }
            }
        )
    }
}
