package io.github.untitled572.pichub.android

import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clipToBounds
import androidx.compose.ui.layout.Layout
import androidx.compose.ui.unit.Constraints
import androidx.compose.ui.unit.dp
import kotlin.math.roundToInt

@Composable
internal fun ViewerLayout(
    landscape: Boolean,
    hasDetails: Boolean,
    stableInsets: WindowInsets,
    reveal: () -> Float,
    image: @Composable () -> Unit,
    panel: @Composable () -> Unit,
    modifier: Modifier = Modifier
) {
    Layout(
        modifier = modifier.clipToBounds(),
        content = { image(); panel() }
    ) { children, constraints ->
        val width = constraints.maxWidth
        val height = constraints.maxHeight
        val top = stableInsets.getTop(this)
        val bottom = stableInsets.getBottom(this)
        val left = stableInsets.getLeft(this, layoutDirection)
        val right = stableInsets.getRight(this, layoutDirection)
        val progress = reveal().coerceIn(0f, 1f)

        val panelSize = if (landscape) {
            val available = (width - left - right).coerceAtLeast(0)
            val desired = if (hasDetails) (available * 0.28f).roundToInt()
                .coerceIn(168.dp.roundToPx(), 224.dp.roundToPx()) else 56.dp.roundToPx()
            Constraints.fixed(
                desired.coerceAtMost((available * 0.53f).roundToInt()),
                (height - top - bottom).coerceAtLeast(0)
            )
        } else {
            Constraints.fixedWidth(width).copy(maxHeight = (height - top).coerceAtLeast(0))
        }
        val controls = children[1].measure(panelSize)

        val imageX = if (landscape) (left * progress).roundToInt() else 0
        val imageY = if (landscape) 0 else (top * progress).roundToInt()
        val panelX = if (landscape) width - ((right + controls.width) * progress).roundToInt() else 0
        val panelY = if (landscape) top else height - (controls.height * progress).roundToInt()
        val photo = children[0].measure(Constraints.fixed(
            (if (landscape) panelX - imageX else width).coerceAtLeast(0),
            (if (landscape) height else panelY - imageY).coerceAtLeast(0)
        ))

        layout(width, height) {
            photo.place(imageX, imageY)
            if (progress > 0f) controls.place(panelX, panelY)
        }
    }
}
