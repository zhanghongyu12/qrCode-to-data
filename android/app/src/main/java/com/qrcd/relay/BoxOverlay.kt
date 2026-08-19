package com.qrcd.relay

import android.content.Context
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.Paint
import android.util.AttributeSet
import android.view.View

/**
 * 在相机预览上绘制扫码识别框高亮。
 * 由 MainActivity 设置最新一帧的归一化坐标（0~1）。
 */
class BoxOverlay @JvmOverloads constructor(
    context: Context, attrs: AttributeSet? = null
) : View(context, attrs) {

    private val paint = Paint().apply {
        color = Color.parseColor("#55dd99")
        style = Paint.Style.STROKE
        strokeWidth = 8f
        isAntiAlias = true
    }

    // 归一化坐标（相对预览宽高）
    @Volatile private var box: FloatArray? = null

    fun setBox(x: Float, y: Float, w: Float, h: Float) {
        box = floatArrayOf(x, y, w, h)
        postInvalidate()
    }

    fun clearBox() {
        box = null
        postInvalidate()
    }

    override fun onDraw(canvas: Canvas) {
        super.onDraw(canvas)
        val b = box ?: return
        val pw = width.toFloat()
        val ph = height.toFloat()
        canvas.drawRect(b[0] * pw, b[1] * ph, (b[0] + b[2]) * pw, (b[1] + b[3]) * ph, paint)
    }
}
