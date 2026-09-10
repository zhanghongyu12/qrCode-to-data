package com.qrcd.relay

import android.Manifest
import android.content.pm.PackageManager
import android.graphics.Bitmap
import android.media.MediaMetadataRetriever
import android.os.Bundle
import android.util.Log
import android.util.Size
import android.view.View
import android.widget.Button
import android.widget.LinearLayout
import android.widget.TextView
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
    private lateinit var usbSwitch: android.widget.Switch
    private lateinit var liveSwitch: android.widget.Switch

    // 实时流式传输（边扫边传 / 录像抽帧传图）共享状态
    @Volatile private var liveUpload = false     // 实时扫描边扫边传激活中
    @Volatile private var liveUrl = ""           // 实时扫描目标 URL（/api/ingest）
    @Volatile private var liveDone = false       // 本次流式会话已完成
    @Volatile private var uploadedCount = 0
    @Volatile private var recvTotalLive = 0
    private val inflight = java.util.concurrent.atomic.AtomicInteger(0)
    private var videoStreaming = false           // 录像抽帧传图进行中
    @Volatile private var failCount = 0          // 连续失败计数（判断开）
    private lateinit var infoText: android.widget.TextView
    private lateinit var hintText: android.widget.TextView
    private lateinit var progressBar: android.widget.ProgressBar
    private lateinit var scanBtn: android.widget.Button
    private lateinit var recordBtn: android.widget.Button
    private lateinit var sendBtn: android.widget.Button
    private lateinit var clearBtn: android.widget.Button

    private val cameraExecutor = Executors.newSingleThreadExecutor()
    // 录像后离线解析用独立单线程池，避免与实时扫描/提交争用
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
    // 扫描期间缓存的帧字节（去重后），点「提交到电脑」批量提交
    private val buf = java.util.concurrent.CopyOnWriteArrayList<ByteArray>()
    @Volatile private var sending = false

    // 传输历史：每次「提交到电脑」成功后归档，可重新提交
    data class TransferRecord(
        val id: Long,
        val label: String,
        val frames: List<ByteArray>,
        val dataCount: Int,
        var status: String
    )
    private val history = ArrayList<TransferRecord>()
    private var historySeq = 0L
    private lateinit var historyList: android.widget.LinearLayout

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
        usbSwitch = findViewById(R.id.usbSwitch)
        usbSwitch.setOnCheckedChangeListener { _, checked ->
            recvUrlEdit.isEnabled = !checked
            if (checked) {
                hintText.text = "USB 直连：数据走 USB 线到接收端电脑"
                testUsbConnection()
            }
        }
        liveSwitch = findViewById(R.id.liveSwitch)
        liveSwitch.setOnCheckedChangeListener { _, checked ->
            if (checked) hintText.text = "实时传输：扫描时边扫边发到接收端"
        }
        infoText = findViewById(R.id.info)
        hintText = findViewById(R.id.hint)
        progressBar = findViewById(R.id.progress)
        scanBtn = findViewById(R.id.scanBtn)
        recordBtn = findViewById(R.id.recordBtn)
        sendBtn = findViewById(R.id.sendBtn)
        clearBtn = findViewById(R.id.clearBtn)
        historyList = findViewById(R.id.historyList)
        loadHistory()
        renderHistory()

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
        hintText.text = ""
        if (liveSwitch.isChecked) {
            val url = resolveRecvUrl()
            if (url.isEmpty()) {
                scanning = false
                scanBtn.text = "开始扫描"
                return
            }
            liveUpload = true
            liveUrl = url
            liveDone = false
            uploadedCount = 0
            recvTotalLive = 0
            sendBtn.isEnabled = false
            infoText.text = "实时传输中… 对准播放端二维码，边扫边发"
        } else {
            liveUpload = false
            liveUrl = ""
            infoText.text = "扫描中… 对准播放端二维码"
        }
        bindCamera()
    }

    private fun stopScan() {
        scanning = false
        scanBtn.text = "开始扫描"
        infoText.text = if (buf.isNotEmpty())
            "已停止，共捕获 ${buf.size} 块，点「提交到电脑」提交"
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
                .setTargetResolution(Size(1920, 1080))
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
                .setQualitySelector(QualitySelector.from(Quality.FHD))
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
        if (parsing || videoStreaming) return
        val url = resolveRecvUrl()
        if (url.isEmpty()) {
            infoText.text = "请先打开「USB 直连」或填写电脑地址，再录像"
            return
        }
        parsing = true
        videoStreaming = true
        liveDone = false
        uploadedCount = 0
        recvTotalLive = 0
        parseExecutor.execute {
            val retriever = MediaMetadataRetriever()
            try {
                retriever.setDataSource(file.absolutePath)
                val durationMs = retriever.extractMetadata(
                    MediaMetadataRetriever.METADATA_KEY_DURATION
                )?.toLongOrNull() ?: 0L
                // 步进约 66ms ≈ 15fps。抽帧用 MediaCodec（快）留在手机，QR 解码也用
                // ML Kit（对真实拍摄的屏幕照片稳健）留在手机；解出的原始帧字节流式发接收端。
                val stepUs = 66_666L
                val durationUs = durationMs * 1000L
                val totalFrames = if (durationUs > 0) (durationUs / stepUs).toInt() else 0
                Log.i(TAG, "parseVideo start: url=$url size=${file.length()} dur=${durationMs}ms frames=$totalFrames")

                var timeUs = 0L
                var frameIndex = 0
                while (timeUs <= durationUs && !liveDone) {
                    val bitmap = retriever.getFrameAtTime(
                        timeUs, MediaMetadataRetriever.OPTION_CLOSEST
                    )
                    if (bitmap != null) {
                        decodeAndStream(bitmap, url, file.name)
                        bitmap.recycle()
                    }
                    frameIndex++
                    // 定期刷新 UI（每 10 帧），避免频繁 runOnUiThread
                    if (frameIndex % 10 == 0) {
                        val pc = capturedCount
                        val up = uploadedCount
                        val rt = recvTotalLive
                        runOnUiThread {
                            infoText.text = "解析传输中… 第 $frameIndex/$totalFrames 帧，已解 $pc 块，还原端已收 $up/$rt"
                        }
                    }
                    timeUs = frameIndex * stepUs
                }
                // 录像文件解析完后可删除以释放空间
                Log.i(TAG, "parseVideo end: frames=$frameIndex captured=$capturedCount")
                if (file.exists()) file.delete()
                if (!liveDone) {
                    runOnUiThread {
                        infoText.text = "解析结束，还原端未完成（可能漏帧，可重录或重试）"
                    }
                }
            } catch (e: Exception) {
                Log.w(TAG, "parseVideo", e)
                runOnUiThread { infoText.text = "解析失败: ${e.message}" }
            } finally {
                retriever.release()
                parsing = false
                videoStreaming = false
            }
        }
    }

    /** 抽帧 → ML Kit 解码（稳健）→ 去重缓存 + 流式发送帧字节到接收端。 */
    private fun decodeAndStream(bitmap: Bitmap, url: String, source: String) {
        val inputImage = InputImage.fromBitmap(bitmap, 0)
        try {
            val barcodes = Tasks.await(scanner.process(inputImage))
            if (barcodes.isNotEmpty()) {
                Log.i(TAG, "decodeAndStream: ${bitmap.width}x${bitmap.height} barcodes=${barcodes.size}")
            }
            for (b in barcodes) {
                if (b.format == Barcode.FORMAT_QR_CODE) {
                    var bytes = b.rawBytes
                    if (bytes == null || bytes.isEmpty()) {
                        val v = b.rawValue
                        if (v != null && v.isNotEmpty()) bytes = base64Decode(v)
                    }
                    Log.i(TAG, "decodeAndStream: qr rawBytes=${bytes?.size ?: -1}")
                    if (bytes != null && bytes.isNotEmpty() && ingestBarcodeBytes(bytes)) {
                        if (url.isNotEmpty() && !liveDone) {
                            streamToReceiver(url, bytes) { onVideoComplete() }
                        }
                    }
                }
            }
        } catch (e: Exception) {
            Log.w(TAG, "decodeAndStream", e)
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
                // 多码网格：一帧可能含多个 QR，全部处理（去重在 ingestBarcodeBytes 内）。
                for (b in barcodes) {
                    if (b.format == Barcode.FORMAT_QR_CODE) {
                        handleBarcode(b, imageProxy.width, imageProxy.height, imageProxy.imageInfo.rotationDegrees)
                    }
                }
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

        // 实时传输：新帧立刻发到接收端
        if (liveUpload && liveUrl.isNotEmpty() && !liveDone) {
            streamToReceiver(liveUrl, bytes) { onLiveScanComplete() }
        }

        // 识别框归一化坐标
        val bb = barcode.boundingBox
        runOnUiThread {
            if (bb != null) {
                overlay.setBox(
                    bb.left.toFloat() / imgW, bb.top.toFloat() / imgH,
                    bb.width().toFloat() / imgW, bb.height().toFloat() / imgH
                )
            }
            if (liveUpload) {
                infoText.text = "实时传输中… 已扫 $capturedCount 块，还原端已收 $uploadedCount/$recvTotalLive"
            } else if (!done && !sending) {
                infoText.text = "✓ 已捕获 $capturedCount 块（点「提交到电脑」提交）"
                sendBtn.isEnabled = true
            }
        }
    }

    // ========== 共享去重+缓存逻辑（实时扫描与录像解析共用） ==========

    /**
     * 对帧字节做去重并缓存到 buf。
     * 返回 true 表示新帧（非重复），false 表示重复丢弃。
     * 仅数据帧(type=0x02)计入 capturedCount（与还原端 TotalSymbols 对齐）。
     */
    private fun ingestBarcodeBytes(bytes: ByteArray): Boolean {
        val key = dedupKey(bytes)
        synchronized(seen) {
            if (!seen.add(key)) return false
        }
        // 帧头偏移 5 是 type：0x02=数据帧，0x01=元数据帧（控制帧，提交但不计数）
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
     * 通用提交：把 frames 逐帧 POST 到 url（/api/ingest）。
     * 每帧响应在 UI 线程刷新进度；结束后回调 onFinish(success, completed)。
     */
    private fun submitFrames(
        url: String,
        frames: List<ByteArray>,
        onFinish: (success: Boolean, completed: Boolean) -> Unit
    ) {
        httpExecutor.execute {
            var sentCount = 0
            var recvTotal = 0
            var lastErr: String? = null
            var completed = false
            var stop = false
            var i = 0
            val total = frames.size
            while (i < total && !stop) {
                try {
                    val req = Request.Builder()
                        .url(url)
                        .post(frames[i].toRequestBody("application/octet-stream".toMediaType()))
                        .build()
                    http.newCall(req).execute().use { resp ->
                        val body = resp.body?.string() ?: ""
                        val j = if (body.isNotEmpty()) JSONObject(body) else JSONObject()
                        val count = j.optInt("count", 0)
                        if (j.optInt("total", 0) > 0) recvTotal = j.optInt("total", 0)
                        if (count > sentCount) sentCount = count
                        val d = j.optBoolean("done", false)
                        val ok = j.optBoolean("ok", true)
                        val err = j.optString("error", "")
                        if (d) completed = true
                        runOnUiThread {
                            if (!ok && err.isNotEmpty()) hintText.text = "还原端错误: $err"
                            if (recvTotal > 0) progressBar.progress = (sentCount * 100 / recvTotal).coerceIn(0, 100)
                            infoText.text = "提交中 ${i + 1}/$total（还原端已收 $sentCount/$recvTotal）"
                        }
                    }
                    if (completed) stop = true
                } catch (e: Exception) {
                    lastErr = e.message
                    runOnUiThread {
                        infoText.text = "提交失败: ${e.message}（已发 ${i + 1}/$total），检查还原端地址后重试"
                    }
                    stop = true
                }
                i++
            }
            runOnUiThread { onFinish(lastErr == null, completed) }
        }
    }

    /** 提交当前扫描缓存；成功后归档到历史并复位，可继续扫下一个文件。 */
    private fun sendBuffer() {
        if (sending) return
        if (buf.isEmpty()) {
            infoText.text = "没有可提交的帧，先扫描"
            return
        }
        val url = resolveRecvUrl()
        if (url.isEmpty()) return
        // 提交前先停扫描，避免提交期间继续往 buf 追加新帧
        scanning = false
        scanBtn.text = "开始扫描"
        try { ProcessCameraProvider.getInstance(this).get().unbindAll() } catch (e: Exception) { }
        sending = true
        sendBtn.isEnabled = false
        scanBtn.isEnabled = false
        recordBtn.isEnabled = false
        val frames = ArrayList(buf)   // 快照，提交期间复位 buf 不受影响
        val total = frames.size
        infoText.text = "提交中… 0/$total"
        submitFrames(url, frames) { success, completed ->
            sending = false
            scanBtn.isEnabled = true
            recordBtn.isEnabled = true
            if (success && completed) {
                archiveCurrent(frames)
            } else {
                sendBtn.isEnabled = true  // 未完成可重试
                if (success) infoText.text = "提交完毕 ($total 块)，还原端未报告完成，可点「提交到电脑」重试"
            }
        }
    }

    /** 提交成功后：归档到历史 + 复位当前会话，准备接收下一个文件。 */
    private fun archiveCurrent(frames: List<ByteArray>) {
        historySeq++
        val rec = TransferRecord(
            id = historySeq,
            label = java.text.SimpleDateFormat("HH:mm:ss", java.util.Locale.getDefault()).format(java.util.Date()),
            frames = ArrayList(frames),
            dataCount = capturedCount,
            status = "已完成"
        )
        history.add(0, rec)
        persistHistory()
        synchronized(seen) { seen.clear() }
        buf.clear()
        capturedCount = 0
        done = false
        progressBar.progress = 0
        sendBtn.isEnabled = false
        scanning = false
        scanBtn.text = "开始扫描"
        try { ProcessCameraProvider.getInstance(this).get().unbindAll() } catch (e: Exception) { }
        overlay.clearBox()
        infoText.text = "✓ 还原完成，已存入记录 #${rec.id}（${rec.dataCount} 块），可继续扫下一个"
        hintText.text = "文件已落在还原端电脑的 downloads/ 目录"
        renderHistory()
    }

    /** 流式上传一帧/一图到接收端（有界并发，喷泉码兜底丢帧）。 */
    private fun streamToReceiver(url: String, body: ByteArray, onDone: () -> Unit) {
        if (liveDone) return
        if (inflight.get() >= MAX_INFLIGHT) return
        inflight.incrementAndGet()
        httpExecutor.execute {
            try {
                val req = Request.Builder()
                    .url(url)
                    .post(body.toRequestBody("application/octet-stream".toMediaType()))
                    .build()
                http.newCall(req).execute().use { resp ->
                    val rbody = resp.body?.string() ?: ""
                    val j = if (rbody.isNotEmpty()) JSONObject(rbody) else JSONObject()
                    val count = j.optInt("count", 0)
                    val total = j.optInt("total", 0)
                    val d = j.optBoolean("done", false)
                    runOnUiThread {
                        failCount = 0
                        if (total > 0) recvTotalLive = total
                        if (count > 0) uploadedCount = count
                        if (recvTotalLive > 0) {
                            progressBar.progress = (uploadedCount * 100 / recvTotalLive).coerceIn(0, 100)
                        }
                        if (d && !liveDone) {
                            liveDone = true
                            onDone()
                        }
                    }
                }
            } catch (e: Exception) {
                // 连续失败达到阈值：判定 USB/网络断开，保帧并提示改走 WiFi
                failCount++
                if (failCount >= FAIL_THRESHOLD && !liveDone) {
                    runOnUiThread {
                        if (!liveDone) {
                            liveDone = true
                            onStreamFailure()
                        }
                    }
                }
            } finally {
                inflight.decrementAndGet()
            }
        }
    }

    /** 实时扫描流式完成：归档已解码帧 + 复位。 */
    private fun onLiveScanComplete() {
        archiveCurrent(ArrayList(buf))
        resetStreamState()
    }

    /** 录像解析传输完成：归档已解码帧 + 复位（与实时扫描一致，可重发）。 */
    private fun onVideoComplete() {
        archiveCurrent(ArrayList(buf))
        resetStreamState()
    }

    /** 复位流式状态（liveDone 由下次会话开始时清空，防止迟到响应重复触发）。 */
    private fun resetStreamState() {
        liveUpload = false
        liveUrl = ""
        uploadedCount = 0
        recvTotalLive = 0
        videoStreaming = false
    }

    /** USB 直连连通性检测：ping 接收端 localhost:8080。 */
    private fun testUsbConnection() {
        infoText.text = "检测 USB 直连…"
        httpExecutor.execute {
            try {
                val req = Request.Builder().url("http://localhost:8080/api/ping").get().build()
                http.newCall(req).execute().use { resp ->
                    val ok = resp.isSuccessful
                    runOnUiThread {
                        if (ok) {
                            infoText.text = "✓ USB 直连正常，点「开始扫描」对准播放端二维码"
                            hintText.text = "USB 直连：数据走 USB 线到接收端电脑"
                        } else {
                            infoText.text = "✗ USB 直连失败（HTTP ${resp.code}），请检查接收端或改填地址走 WiFi"
                            hintText.text = "USB 不通时：关掉 USB 开关，填电脑地址走 WiFi"
                        }
                    }
                }
            } catch (e: Exception) {
                runOnUiThread {
                    infoText.text = "✗ USB 直连不通——请确认：接收端已开、手机开 USB 调试、线是数据线；或改走 WiFi"
                    hintText.text = "USB 不通时：关掉 USB 开关，填电脑地址走 WiFi"
                }
            }
        }
    }

    /** 流式传输失败：停止流式，保留本地帧，允许切 WiFi 批量重发。 */
    private fun onStreamFailure() {
        if (videoStreaming) {
            videoStreaming = false
            sendBtn.isEnabled = true
            infoText.text = "传输失败（已缓存 $capturedCount 块）——可切 WiFi 后点「提交到电脑」重发"
            hintText.text = "或从历史记录重新提交"
        } else {
            scanning = false
            scanBtn.text = "开始扫描"
            try { ProcessCameraProvider.getInstance(this).get().unbindAll() } catch (e: Exception) { }
            overlay.clearBox()
            liveUpload = false
            sendBtn.isEnabled = true
            infoText.text = "传输失败（已缓存 $capturedCount 块）——关掉 USB、填电脑地址后点「提交到电脑」重发"
            hintText.text = "或直接点「提交到电脑」用当前地址重试"
        }
    }

    /** 从历史重新提交某条记录。 */
    private fun resubmit(rec: TransferRecord) {
        val url = resolveRecvUrl()
        if (url.isEmpty()) return
        rec.status = "提交中"
        renderHistory()
        infoText.text = "重新提交记录 #${rec.id} …"
        submitFrames(url, rec.frames) { success, completed ->
            rec.status = if (success && completed) "已完成" else "待提交"
            persistHistory()
            renderHistory()
            infoText.text = if (completed) "✓ 记录 #${rec.id} 重新提交完成"
                else "记录 #${rec.id} 重新提交未完成，可再次重试"
        }
    }

    /** 重建历史列表 UI。 */
    private fun renderHistory() {
        historyList.removeAllViews()
        if (history.isEmpty()) {
            val t = TextView(this)
            t.text = "暂无记录"
            t.setTextColor(0xff888888.toInt())
            t.textSize = 12f
            historyList.addView(t)
            return
        }
        for (rec in history) {
            val row = LinearLayout(this)
            row.orientation = LinearLayout.HORIZONTAL
            row.gravity = android.view.Gravity.CENTER_VERTICAL
            row.setPadding(0, 4, 0, 4)
            val info = TextView(this)
            info.text = "#${rec.id} ${rec.label} · ${rec.dataCount} 块 · ${rec.status}"
            info.setTextColor(0xffcccccc.toInt())
            info.textSize = 12f
            info.layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
            val btn = Button(this)
            btn.text = "重新提交"
            btn.textSize = 12f
            btn.setOnClickListener { resubmit(rec) }
            row.addView(info)
            row.addView(btn)
            historyList.addView(row)
        }
    }

    /** 帧文件格式：每帧 4 字节大端长度 + 帧字节，逐帧拼接。 */
    private fun writeFrames(file: File, frames: List<ByteArray>) {
        file.outputStream().use { out ->
            val lb = ByteArray(4)
            for (f in frames) {
                val n = f.size
                lb[0] = (n ushr 24).toByte()
                lb[1] = (n ushr 16).toByte()
                lb[2] = (n ushr 8).toByte()
                lb[3] = n.toByte()
                out.write(lb)
                out.write(f)
            }
        }
    }

    private fun readFrames(file: File): List<ByteArray> {
        if (!file.exists()) return emptyList()
        val b = file.readBytes()
        val list = ArrayList<ByteArray>()
        var i = 0
        while (i + 4 <= b.size) {
            val n = ((b[i].toInt() and 0xff) shl 24) or
                ((b[i + 1].toInt() and 0xff) shl 16) or
                ((b[i + 2].toInt() and 0xff) shl 8) or
                (b[i + 3].toInt() and 0xff)
            i += 4
            if (n <= 0 || i + n > b.size) break
            list.add(b.copyOfRange(i, i + n))
            i += n
        }
        return list
    }

    /** 把全部历史写盘（帧文件 + index.json），App 重启后不丢。 */
    private fun persistHistory() {
        try {
            val dir = File(filesDir, "history")
            dir.mkdirs()
            val arr = org.json.JSONArray()
            for (rec in history) {
                writeFrames(File(dir, "f_${rec.id}.bin"), rec.frames)
                val o = org.json.JSONObject()
                o.put("id", rec.id)
                o.put("label", rec.label)
                o.put("dataCount", rec.dataCount)
                o.put("status", rec.status)
                arr.put(o)
            }
            File(dir, "index.json").writeText(arr.toString())
        } catch (e: Exception) {
            Log.w(TAG, "persistHistory failed", e)
        }
    }

    /** 启动时从盘上恢复历史。 */
    private fun loadHistory() {
        history.clear()
        try {
            val dir = File(filesDir, "history")
            val idx = File(dir, "index.json")
            if (!idx.exists()) return
            val arr = org.json.JSONArray(idx.readText())
            for (i in 0 until arr.length()) {
                val o = arr.getJSONObject(i)
                val id = o.optLong("id")
                val frames = readFrames(File(dir, "f_${id}.bin"))
                if (frames.isEmpty()) continue
                history.add(
                    TransferRecord(
                        id = id,
                        label = o.optString("label"),
                        frames = frames,
                        dataCount = o.optInt("dataCount"),
                        status = o.optString("status", "已完成")
                    )
                )
            }
            historySeq = history.maxOfOrNull { it.id } ?: 0L
        } catch (e: Exception) {
            Log.w(TAG, "loadHistory failed", e)
        }
    }

    private fun resolveRecvUrl(): String {
        if (usbSwitch.isChecked) {
            // USB 直连：经 adb reverse 隧道走 USB 线，POST 到本机映射的 8080
            return "http://localhost:8080/api/ingest"
        }
        var s = recvUrlEdit.text.toString().trim()
        if (s.isEmpty()) {
            infoText.text = "请先填写电脑地址（本机IP:端口，如 192.168.253.1:8080）"
            return ""
        }
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
        private const val MAX_INFLIGHT = 32
        private const val FAIL_THRESHOLD = 3
    }
}
