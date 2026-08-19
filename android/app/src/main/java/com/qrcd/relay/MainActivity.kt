package com.qrcd.relay

import android.Manifest
import android.content.pm.PackageManager
import android.os.Bundle
import android.util.Log
import android.view.View
import androidx.appcompat.app.AppCompatActivity
import androidx.camera.core.CameraSelector
import androidx.camera.core.ImageAnalysis
import androidx.camera.core.ImageProxy
import androidx.camera.core.Preview
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.camera.view.PreviewView
import androidx.core.app.ActivityCompat
import androidx.core.content.ContextCompat
import com.google.mlkit.vision.barcode.BarcodeScannerOptions
import com.google.mlkit.vision.barcode.BarcodeScanning
import com.google.mlkit.vision.barcode.common.Barcode
import com.google.mlkit.vision.common.InputImage
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit

class MainActivity : AppCompatActivity() {

    private lateinit var previewView: PreviewView
    private lateinit var overlay: BoxOverlay
    private lateinit var recvUrlEdit: android.widget.EditText
    private lateinit var infoText: android.widget.TextView
    private lateinit var hintText: android.widget.TextView
    private lateinit var progressBar: android.widget.ProgressBar
    private lateinit var scanBtn: android.widget.Button
    private lateinit var clearBtn: android.widget.Button

    private val cameraExecutor = Executors.newSingleThreadExecutor()
    private val http = OkHttpClient.Builder()
        .connectTimeout(5, TimeUnit.SECONDS)
        .readTimeout(10, TimeUnit.SECONDS)
        .writeTimeout(10, TimeUnit.SECONDS)
        .build()

    private val scanner = BarcodeScanning.getClient(
        BarcodeScannerOptions.Builder()
            .setBarcodeFormats(Barcode.FORMAT_QR_CODE)
            .build()
    )

    @Volatile private var scanning = false
    @Volatile private var done = false
    private val seen = HashSet<Long>()   // 已转发帧的去重键
    private var analyzeBusy = false

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        previewView = findViewById(R.id.preview)
        overlay = findViewById(R.id.overlay)
        recvUrlEdit = findViewById(R.id.recvUrl)
        infoText = findViewById(R.id.info)
        hintText = findViewById(R.id.hint)
        progressBar = findViewById(R.id.progress)
        scanBtn = findViewById(R.id.scanBtn)
        clearBtn = findViewById(R.id.clearBtn)

        scanBtn.setOnClickListener {
            if (scanning) stopScan() else startScan()
        }
        clearBtn.setOnClickListener {
            synchronized(seen) { seen.clear() }
            progressBar.progress = 0
            done = false
            infoText.text = "已清空，重新扫描"
            hintText.text = ""
        }
    }

    private fun startScan() {
        if (ContextCompat.checkSelfPermission(this, Manifest.permission.CAMERA)
            != PackageManager.PERMISSION_GRANTED
        ) {
            ActivityCompat.requestPermissions(this, arrayOf(Manifest.permission.CAMERA), 1)
            return
        }
        scanning = true
        done = false
        scanBtn.text = "停止扫描"
        infoText.text = "扫描中… 对准发送端二维码"
        hintText.text = ""
        bindCamera()
    }

    private fun stopScan() {
        scanning = false
        scanBtn.text = "开始扫描"
        infoText.text = "已停止"
        overlay.clearBox()
        try {
            val provider = ProcessCameraProvider.getInstance(this).get()
            provider.unbindAll()
        } catch (e: Exception) {
            Log.w(TAG, "unbind", e)
        }
    }

    private fun bindCamera() {
        val providerFuture = ProcessCameraProvider.getInstance(this)
        providerFuture.addListener({
            val provider = providerFuture.get()
            val preview = Preview.Builder().build().also {
                it.setSurfaceProvider(previewView.surfaceProvider)
            }
            val analysis = ImageAnalysis.Builder()
                .setBackpressureStrategy(ImageAnalysis.STRATEGY_KEEP_ONLY_LATEST)
                .build()
                .also { it.setAnalyzer(cameraExecutor, ::analyzeFrame) }

            try {
                provider.unbindAll()
                provider.bindToLifecycle(this, CameraSelector.DEFAULT_BACK_CAMERA, preview, analysis)
            } catch (e: Exception) {
                infoText.text = "相机绑定失败: ${e.message}"
            }
        }, ContextCompat.getMainExecutor(this))
    }

    /**
     * ImageAnalysis 回调：ML Kit 解码 → 去重 → POST 转发。
     * ML Kit 对 QR byte-mode 返回 rawBytes（二进制帧字节），与发送端帧字节一致，
     * 直接 POST 给接收端 /api/ingest，无需 base64 绕路。
     */
    private fun analyzeFrame(imageProxy: ImageProxy) {
        if (!scanning || done || analyzeBusy) {
            imageProxy.close()
            return
        }
        analyzeBusy = true
        val mediaImage = imageProxy.image
        if (mediaImage == null) {
            imageProxy.close()
            analyzeBusy = false
            return
        }
        val inputImage = InputImage.fromMediaImage(mediaImage, imageProxy.imageInfo.rotationDegrees)
        scanner.process(inputImage)
            .addOnSuccessListener { barcodes ->
                val b = barcodes.firstOrNull { it.format == Barcode.FORMAT_QR_CODE }
                if (b != null) handleBarcode(b, imageProxy.width, imageProxy.height, imageProxy.imageInfo.rotationDegrees)
            }
            .addOnCompleteListener {
                imageProxy.close()
                analyzeBusy = false
            }
    }

    private fun handleBarcode(barcode: Barcode, imgW: Int, imgH: Int, rotation: Int) {
        val bytes = barcode.rawBytes
        if (bytes == null || bytes.isEmpty()) {
            runOnUiThread { hintText.text = "解码为空，请重试" }
            return
        }
        // 去重键：长度 + 前 16 字节的简单 hash
        val key = dedupKey(bytes)
        synchronized(seen) {
            if (!seen.add(key)) return
        }
        // 识别框归一化坐标
        val bb = barcode.boundingBox
        if (bb != null) {
            runOnUiThread {
                overlay.setBox(
                    bb.left.toFloat() / imgW, bb.top.toFloat() / imgH,
                    bb.width().toFloat() / imgW, bb.height().toFloat() / imgH
                )
            }
        }
        postFrame(bytes)
    }

    private fun dedupKey(b: ByteArray): Long {
        val n = b.size
        val lim = minOf(16, n)
        var h = n.toLong()
        for (i in 0 until lim) {
            h = h * 31 + (b[i].toInt() and 0xFF)
        }
        return h
    }

    private fun postFrame(bytes: ByteArray) {
        val url = resolveRecvUrl()
        val req = Request.Builder()
            .url(url)
            .post(bytes.toRequestBody("application/octet-stream".toMediaType()))
            .build()
        cameraExecutor.execute {
            try {
                http.newCall(req).execute().use { resp ->
                    val body = resp.body?.string() ?: ""
                    val j = if (body.isNotEmpty()) JSONObject(body) else JSONObject()
                    val count = j.optInt("count", 0)
                    val total = j.optInt("total", 0)
                    val isDone = j.optBoolean("done", false)
                    val ok = j.optBoolean("ok", true)
                    val err = j.optString("error", "")
                    runOnUiThread {
                        if (!ok && err.isNotEmpty()) {
                            infoText.text = "接收端错误: $err"
                            return@runOnUiThread
                        }
                        if (total > 0) {
                            progressBar.progress = (count * 100 / total).coerceIn(0, 100)
                        }
                        if (isDone) {
                            done = true
                            infoText.text = "✓ 还原完成 ($count/$total)"
                            hintText.text = "文件已落在接收端 PC 的 downloads/ 目录"
                            scanBtn.text = "开始扫描"
                            scanning = false
                            overlay.clearBox()
                        } else {
                            infoText.text = "已上传 $count/$total 块"
                        }
                    }
                }
            } catch (e: Exception) {
                runOnUiThread {
                    infoText.text = "上传失败: ${e.message}（检查接收端地址）"
                }
            }
        }
    }

    private fun resolveRecvUrl(): String {
        var s = recvUrlEdit.text.toString().trim()
        if (s.isEmpty()) s = "localhost:8080"
        if (!s.startsWith("http")) s = "http://$s"
        s = s.trimEnd('/')
        if (!s.endsWith("/api/ingest")) s = "$s/api/ingest"
        return s
    }

    override fun onDestroy() {
        super.onDestroy()
        scanning = false
        cameraExecutor.shutdown()
        scanner.close()
    }

    override fun onRequestPermissionsResult(
        requestCode: Int, permissions: Array<String>, grantResults: IntArray
    ) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults)
        if (requestCode == 1 && grantResults.isNotEmpty() && grantResults[0] == PackageManager.PERMISSION_GRANTED) {
            startScan()
        } else {
            infoText.text = "需要相机权限才能扫码"
        }
    }

    companion object {
        private const val TAG = "qrcd.relay"
    }
}
