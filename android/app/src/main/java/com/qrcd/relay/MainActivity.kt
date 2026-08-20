package com.qrcd.relay

import android.Manifest
import android.content.pm.PackageManager
import android.graphics.Bitmap
import android.media.MediaMetadataRetriever
import android.os.Bundle
import android.util.Log
import android.view.View
import androidx.appcompat.app.AppCompatActivity
import androidx.camera.core.CameraSelector
import androidx.camera.core.ImageAnalysis
import androidx.camera.core.ImageProxy
import androidx.camera.core.Preview
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.camera.video.FileOutputOptions
import androidx.camera.video.Quality
import androidx.camera.video.QualitySelector
import androidx.camera.video.Recorder
import androidx.camera.video.Recording
import androidx.camera.video.VideoCapture
import androidx.camera.video.VideoRecordEvent
import androidx.camera.view.PreviewView
import androidx.core.app.ActivityCompat
import androidx.core.content.ContextCompat
import com.google.android.gms.tasks.Tasks
import com.google.mlkit.vision.barcode.BarcodeScannerOptions
import com.google.mlkit.vision.barcode.BarcodeScanning
import com.google.mlkit.vision.barcode.common.Barcode
import com.google.mlkit.vision.common.InputImage
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject
import java.io.File
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
    private lateinit var recordBtn: android.widget.Button
    private lateinit var sendBtn: android.widget.Button
    private lateinit var clearBtn: android.widget.Button

    private val cameraExecutor = Executors.newSingleThreadExecutor()
    // 录像后离线解析用独立单线程池，避免与实时扫描/上传争用
    private val parseExecutor = Executors.newSingleThreadExecutor()
    // HTTP 转发用独立线程池，避免阻塞相机分析线程
    private val httpExecutor = Executors.newCachedThreadPool()
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
    @Volatile private var recording = false      // 录像模式激活中
    @Volatile private var parsing = false        // 录像后离线抽帧解析进行中
    @Volatile private var pendingRecord = false  // 权限请求中待录像授权（非扫描）
    @Volatile private var done = false
    private val seen = HashSet<Long>()   // 已捕获帧的去重键
    private var analyzeBusy = false
    @Volatile private var capturedCount = 0   // 本地已捕获（去重后）帧数，即时反馈
    // 扫描期间缓存的帧字节（去重后），点「发送到 PC」批量上传
    private val buf = java.util.concurrent.CopyOnWriteArrayList<ByteArray>()
    @Volatile private var sending = false

    // 录像相关：VideoCapture 绑定、当前 Recording、输出文件、录制序号
    private var videoCapture: VideoCapture<Recorder>? = null
    private var activeRecording: Recording? = null
    private var activeRecordFile: File? = null
    private var recordSeq = 0

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
        recordBtn = findViewById(R.id.recordBtn)
        sendBtn = findViewById(R.id.sendBtn)
        clearBtn = findViewById(R.id.clearBtn)

        scanBtn.setOnClickListener {
            if (scanning) stopScan() else startScan()
        }
        recordBtn.setOnClickListener {
            if (recording) stopRecording() else startRecording()
        }
        sendBtn.setOnClickListener { sendBuffer() }
        clearBtn.setOnClickListener {
            synchronized(seen) { seen.clear() }
            buf.clear()
            capturedCount = 0
            progressBar.progress = 0
            done = false
            sending = false
            sendBtn.isEnabled = true
            recordSeq = 0
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
        infoText.text = if (buf.isNotEmpty())
            "已停止，共捕获 ${buf.size} 块，点「发送到 PC」上传"
        else "已停止，未捕获到任何帧"
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

    // ========== 录像模式 ==========

    private fun startRecording() {
        if (ContextCompat.checkSelfPermission(this, Manifest.permission.CAMERA)
            != PackageManager.PERMISSION_GRANTED
        ) {
            pendingRecord = true
            ActivityCompat.requestPermissions(this, arrayOf(Manifest.permission.CAMERA), 1)
            return
        }
        // 录像与实时扫描互斥：若正在扫描则先行停止
        if (scanning) stopScan()

        recording = true
        recordBtn.text = "停止录像"
        infoText.text = "录像中… 再次点击停止并解析"
        hintText.text = ""

        bindRecordCamera()
    }

    private fun bindRecordCamera() {
        val providerFuture = ProcessCameraProvider.getInstance(this)
        providerFuture.addListener({
            val provider = providerFuture.get()
            val preview = Preview.Builder().build().also {
                it.setSurfaceProvider(previewView.surfaceProvider)
            }
            val recorder = Recorder.Builder()
                .setQualitySelector(QualitySelector.from(Quality.SD))
                .build()
            videoCapture = VideoCapture.withOutput(recorder)

            try {
                provider.unbindAll()
                provider.bindToLifecycle(this, CameraSelector.DEFAULT_BACK_CAMERA, preview, videoCapture!!)
            } catch (e: Exception) {
                infoText.text = "相机绑定失败: ${e.message}"
                recording = false
                recordBtn.text = "录像模式"
                return@addListener
            }

            // 开始录像输出到 app cacheDir，文件名含序号支持多段录制
            val outputFile = File(cacheDir, "qrcd_rec_${recordSeq}.mp4")
            activeRecordFile = outputFile
            val outputOptions = FileOutputOptions.Builder(outputFile).build()
            val videoOutput = videoCapture?.output
            if (videoOutput != null) {
                activeRecording = videoOutput
                    .prepareRecording(this, outputOptions)
                    .start(ContextCompat.getMainExecutor(this)) { event ->
                        onRecordEvent(event, outputFile, provider)
                    }
            }
        }, ContextCompat.getMainExecutor(this))
    }

    private fun onRecordEvent(event: VideoRecordEvent, outputFile: File, provider: ProcessCameraProvider) {
        when (event) {
            is VideoRecordEvent.Finalize -> {
                recording = false
                activeRecording = null
                recordBtn.text = "录像模式"
                // 解绑相机，释放摄像头
                try { provider.unbindAll() } catch (e: Exception) { Log.w(TAG, "unbind after record", e) }
                if (event.error != 0) {
                    runOnUiThread { infoText.text = "录像失败: error=${event.error}" }
                } else {
                    recordSeq++
                    runOnUiThread { infoText.text = "录像完成，开始解析视频…" }
                    parseVideo(outputFile)
                }
            }
            is VideoRecordEvent.Start -> {
                runOnUiThread { infoText.text = "录像中…" }
            }
            else -> { /* ignore status events */ }
        }
    }

    private fun stopRecording() {
        if (recording) {
            val file = activeRecordFile
            if (file != null) {
                infoText.text = "录像停止，等待最终化…"
            }
            activeRecording?.stop()
            activeRecording = null
            // onRecordEvent Finalize 中会继续：解绑相机 + 解析视频
            // 但若 stopRecording 时用户已按下停止，需要确保录制结束
            // 如果录制已由 Finalize 结束，activeRecording 已是 null，这里安全调用
        }
    }

    // ========== 视频离线解析 ==========

    private fun parseVideo(file: File) {
        if (parsing) return
        parsing = true
        parseExecutor.execute {
            val retriever = MediaMetadataRetriever()
            try {
                retriever.setDataSource(file.absolutePath)
                val durationMs = retriever.extractMetadata(
                    MediaMetadataRetriever.METADATA_KEY_DURATION
                )?.toLongOrNull() ?: 0L
                // 步进约 66ms ≈ 15fps，平衡解析速度与帧覆盖率
                val stepUs = 66_666L
                val durationUs = durationMs * 1000L
                val totalFrames = if (durationUs > 0) (durationUs / stepUs).toInt() else 0

                var timeUs = 0L
                var frameIndex = 0
                while (timeUs <= durationUs) {
                    val bitmap = retriever.getFrameAtTime(
                        timeUs, MediaMetadataRetriever.OPTION_CLOSEST
                    )
                    if (bitmap != null) {
                        decodeBarcodeFromBitmap(bitmap, file.name)
                        bitmap.recycle()
                    } else {
                        Log.w(TAG, "parseVideo: null bitmap at $timeUs us")
                    }
                    frameIndex++
                    // 定期刷新 UI（每 10 帧），避免频繁 runOnUiThread
                    if (frameIndex % 10 == 0) {
                        val pc = capturedCount
                        runOnUiThread {
                            infoText.text = "解析中… 第 ${frameIndex}/${totalFrames} 帧，已解析 $pc 块"
                        }
                    }
                    timeUs = frameIndex * stepUs
                }
                // 录像文件解析完后可删除以释放空间
                if (file.exists()) file.delete()
                val pc = capturedCount
                runOnUiThread {
                    infoText.text = "✓ 录像解析完成，共 $pc 块（点「发送到 PC」上传）"
                    sendBtn.isEnabled = true
                }
            } catch (e: Exception) {
                Log.w(TAG, "parseVideo", e)
                runOnUiThread { infoText.text = "解析失败: ${e.message}" }
            } finally {
                retriever.release()
                parsing = false
            }
        }
    }

    /** 将 Bitmap 帧送入 ML Kit 解码，去重+缓存，与实时扫描共用同一路径。 */
    private fun decodeBarcodeFromBitmap(bitmap: Bitmap, source: String) {
        val inputImage = InputImage.fromBitmap(bitmap, 0)
        try {
            val barcodes = Tasks.await(scanner.process(inputImage))
            for (b in barcodes) {
                if (b.format == Barcode.FORMAT_QR_CODE) {
                    val bytes = b.rawBytes
                    if (bytes != null && bytes.isNotEmpty()) {
                        if (ingestBarcodeBytes(bytes)) {
                            // 仅记录日志，不过度刷 UI
                            Log.d(TAG, "decodeBarcodeFromBitmap: new frame from $source")
                        }
                    }
                }
            }
        } catch (e: Exception) {
            Log.w(TAG, "decodeBarcodeFromBitmap", e)
        }
    }

    // ========== 实时扫描（ImageAnalysis 回调） ==========

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
        // ML Kit 对 QR 优先返回 rawBytes（二进制原始帧字节）；部分情况只返回 rawValue（文本）。
        var bytes: ByteArray? = barcode.rawBytes
        if (bytes == null || bytes.isEmpty()) {
            val v = barcode.rawValue
            if (v != null && v.isNotEmpty()) {
                bytes = base64Decode(v)
            }
        }
        if (bytes == null || bytes.isEmpty()) {
            runOnUiThread { hintText.text = "解码为空，请重试" }
            return
        }
        // 去重+缓存（与录像解析共享同一路径）
        if (!ingestBarcodeBytes(bytes)) return

        // 识别框归一化坐标
        val bb = barcode.boundingBox
        runOnUiThread {
            if (bb != null) {
                overlay.setBox(
                    bb.left.toFloat() / imgW, bb.top.toFloat() / imgH,
                    bb.width().toFloat() / imgW, bb.height().toFloat() / imgH
                )
            }
            // 即时反馈：已捕获帧数（暂存本地，点「发送到 PC」上传）
            if (!done && !sending) {
                infoText.text = "✓ 已捕获 $capturedCount 块（点「发送到 PC」上传）"
                sendBtn.isEnabled = true
            }
        }
    }

    // ========== 共享去重+缓存逻辑（实时扫描与录像解析共用） ==========

    /**
     * 对帧字节做去重并缓存到 buf。
     * 返回 true 表示新帧（非重复），false 表示重复丢弃。
     * 仅数据帧(type=0x02)计入 capturedCount（与接收端 TotalSymbols 对齐）。
     */
    private fun ingestBarcodeBytes(bytes: ByteArray): Boolean {
        val key = dedupKey(bytes)
        synchronized(seen) {
            if (!seen.add(key)) return false
        }
        // 帧头偏移 5 是 type：0x02=数据帧，0x01=元数据帧（控制帧，上传但不计数）
        val isData = bytes.size > 5 && (bytes[5].toInt() and 0xFF) == 0x02
        buf.add(bytes)
        if (isData) capturedCount++
        return true
    }

    /** Base64 解码（标准 Android API）。rawValue 路径回退用。 */
    private fun base64Decode(s: String): ByteArray {
        return android.util.Base64.decode(s, android.util.Base64.DEFAULT)
    }

    // 整帧哈希去重。注意：帧头前 16 字节是 magic+version+type+flags+transfer_id 前半，
    // 同一会话的所有数据帧这些字段完全相同，seq 在偏移 24:28，payload 在 32 之后。
    // 早期只哈希前 16 字节+长度 → 所有等长数据帧 dedupKey 相同，导致只剩第一个数据帧被捕获。
    // 改为对全帧做哈希（帧很小，~187B），保证不同 seq 不碰撞。
    private fun dedupKey(b: ByteArray): Long {
        var h = b.size.toLong()
        for (i in b.indices) {
            h = h * 31L + (b[i].toInt() and 0xFF)
        }
        return h
    }

    /**
     * 批量发送已缓存的帧到接收端 /api/ingest，逐帧 POST。
     * 接收端返回 count/total 进度；done=true 表示重组完成。
     */
    private fun sendBuffer() {
        if (sending) return
        if (buf.isEmpty()) {
            infoText.text = "没有可发送的帧，先扫描"
            return
        }
        sending = true
        sendBtn.isEnabled = false
        scanBtn.isEnabled = false
        recordBtn.isEnabled = false
        val total = buf.size
        infoText.text = "发送中… 0/$total"
        val url = resolveRecvUrl()
        httpExecutor.execute {
            var sentCount = 0
            var recvTotal = 0
            var lastErr: String? = null
            var stop = false
            var i = 0
            while (i < buf.size && !stop) {
                try {
                    val req = Request.Builder()
                        .url(url)
                        .post(buf[i].toRequestBody("application/octet-stream".toMediaType()))
                        .build()
                    val isDone = http.newCall(req).execute().use { resp ->
                        val body = resp.body?.string() ?: ""
                        val j = if (body.isNotEmpty()) JSONObject(body) else JSONObject()
                        val count = j.optInt("count", 0)
                        if (j.optInt("total", 0) > 0) recvTotal = j.optInt("total", 0)
                        if (count > sentCount) sentCount = count
                        val d = j.optBoolean("done", false)
                        val ok = j.optBoolean("ok", true)
                        val err = j.optString("error", "")
                        runOnUiThread {
                            if (!ok && err.isNotEmpty()) {
                                hintText.text = "接收端错误: $err"
                                return@runOnUiThread
                            }
                            if (recvTotal > 0) {
                                progressBar.progress = (sentCount * 100 / recvTotal).coerceIn(0, 100)
                            }
                            infoText.text = "发送中 ${i + 1}/$total（接收端已收 $sentCount/$recvTotal）"
                            if (d) {
                                done = true
                                infoText.text = "✓ 还原完成 ($sentCount/$recvTotal)"
                                hintText.text = "文件已落在接收端 PC 的 downloads/ 目录"
                                overlay.clearBox()
                            }
                        }
                        d
                    }
                    if (isDone) stop = true
                } catch (e: Exception) {
                    lastErr = e.message
                    runOnUiThread {
                        infoText.text = "发送失败: ${e.message}（已发 ${i + 1}/$total），检查接收端地址后重试"
                    }
                    stop = true
                }
                i++
            }
            runOnUiThread {
                sending = false
                scanBtn.isEnabled = true
                recordBtn.isEnabled = true
                if (!done) {
                    sendBtn.isEnabled = true  // 允许重试/补发
                    if (lastErr == null) {
                        infoText.text = "发送完毕 ($total 块)；接收端进度 $sentCount/$recvTotal"
                    }
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
        recording = false
        activeRecording?.stop()
        activeRecording = null
        cameraExecutor.shutdown()
        parseExecutor.shutdown()
        httpExecutor.shutdown()
        scanner.close()
    }

    override fun onRequestPermissionsResult(
        requestCode: Int, permissions: Array<String>, grantResults: IntArray
    ) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults)
        if (requestCode == 1 && grantResults.isNotEmpty() && grantResults[0] == PackageManager.PERMISSION_GRANTED) {
            if (pendingRecord) {
                pendingRecord = false
                startRecording()
            } else {
                startScan()
            }
        } else {
            pendingRecord = false
            infoText.text = "需要相机权限才能扫码或录像"
        }
    }

    companion object {
        private const val TAG = "qrcd.relay"
    }
}
