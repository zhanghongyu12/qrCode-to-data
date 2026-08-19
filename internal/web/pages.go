package web

// indexPage 首页：三端入口。
const indexPage = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>qrcd 三端</title>
<style>body{font-family:system-ui,sans-serif;max-width:680px;margin:40px auto;padding:0 20px;color:#222}
a{display:block;padding:20px;margin:12px 0;border:2px solid #2a7;border-radius:12px;text-decoration:none;color:#2a7;font-size:18px}
a:hover{background:#2a7;color:#fff}
small{color:#888}</style></head>
<body>
<h1>qrcd · 二维码数据传输</h1>
<p>三个产物，纯光学（屏幕二维码 ↔ 摄像头），全程不联网：</p>
<a href="/sender">① 发送端 —— 选文件，屏幕播放二维码</a>
<a href="/relay">② 手机中继 —— 扫码后转发给接收端（手机访问，需 HTTPS）</a>
<a href="/receiver">③ 接收端 —— 接收手机上传的帧，重组并下载</a>
<hr><small>发送端电脑 →（手机扫码）→ 手机 →（网络）→ 接收端电脑。接收端无需摄像头。<br>
手机中继须用 <b>https://本机IP:8443/relay</b>（浏览器只允许 HTTPS 开摄像头，首次会提示证书不安全，点继续访问即可）。</small>
</body></html>`

// senderPage 发送端：选文件→提交→屏幕逐帧播放 QR。
const senderPage = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>qrcd 发送端</title>
<style>body{font-family:system-ui,sans-serif;text-align:center;margin:0;padding:20px;background:#111;color:#eee}
h1{font-size:20px} #drop{border:2px dashed #6a6;border-radius:12px;padding:40px;margin:20px auto;max-width:480px;cursor:pointer}
#drop.hover{background:#1a3a1a} #qr{margin:20px auto;width:min(70vmin,460px);height:auto;image-rendering:pixelated;background:#fff}
.bar{background:#333;height:8px;border-radius:4px;margin:12px auto;max-width:400px}
.bar>i{display:block;height:100%;width:0;background:#5d9;border-radius:4px}
input[type=range]{width:240px} button{font-size:16px;padding:10px 24px;border:none;border-radius:8px;background:#2a7;color:#fff;cursor:pointer;margin:6px}
</style></head>
<body>
<h1>qrcd 发送端</h1>
<p>选文件，屏幕逐帧播放二维码，用另一台设备摄像头扫。</p>
<div id="drop"><span id="dlabel">点击或拖入文件</span><input type="file" id="file" hidden></div>
<div><label>FPS <input type="range" id="fps" min="1" max="20" value="6"><span id="fpsv">6</span></label></div>
<button id="send">开始发送</button>
<button id="stop" hidden>停止</button>
<div class="bar"><i id="prog"></i></div>
<canvas id="qr"></canvas>
<div id="info"></div>
<script>
const $=id=>document.getElementById(id);
let frames=0,name='',size=0,playing=false,i=0,timer=null;
$('fps').oninput=e=>$('fpsv').textContent=e.target.value;
$('drop').onclick=()=>$('file').click();
['dragover'].forEach(e=>$('drop').addEventListener(e,ev=>{ev.preventDefault();$('drop').classList.add('hover')}));
['dragleave','drop'].forEach(e=>$('drop').addEventListener(e,ev=>{ev.preventDefault();$('drop').classList.remove('hover')}));
$('drop').addEventListener('drop',ev=>{if(ev.dataTransfer.files[0])$('file').files=ev.dataTransfer.files});
$('file').onchange=e=>{const f=e.target.files[0];if(f)$('dlabel').textContent='已选: '+f.name};
$('send').onclick=async()=>{
  const fd=new FormData();const f=$('file').files[0];
  if(f)fd.append('file',f); else fd.append('text',prompt('输入要发送的文本','')||'');
  $('info').textContent='编码中…';
  const r=await fetch('/api/encode',{method:'POST',body:fd});
  const j=await r.json();
  if(!r.ok){$('info').textContent='错误';return}
  frames=j.frames;name=j.name;size=j.size;i=0;playing=true;
  $('send').hidden=true;$('stop').hidden=false;
  $('info').textContent='文件: '+name+' ('+size+'B), 共 '+frames+' 帧';
  play();
};
$('stop').onclick=()=>{playing=false;clearTimeout(timer);$('send').hidden=false;$('stop').hidden=true;$('info').textContent='已停止'};
function play(){
  if(!playing)return;
  if(i>=frames){$('info').textContent='发送完成 ✓（循环重放中）';i=0}
  const img=new Image();
  img.onload=()=>{const c=$('qr');const tgt=Math.min(window.innerWidth,window.innerHeight)*0.7;
    const sc=Math.max(1,Math.floor(tgt/img.width));c.width=img.width*sc;c.height=img.height*sc;
    const x=c.getContext('2d');x.imageSmoothingEnabled=false;x.drawImage(img,0,0,c.width,c.height);
    $('prog').style.width=(100*i/frames)+'%'};
  img.src='/api/frame/'+i;i++;
  timer=setTimeout(play,1000/parseInt($('fps').value));
}
</script></body></html>`

// relayPage 手机中继：手动「开始扫描」→ 逐帧解码并缓存 → 「发送到 PC」批量 POST 给接收端。
const relayPage = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1,maximum-scale=1">
<title>qrcd 手机中继</title>
<style>body{font-family:system-ui,sans-serif;text-align:center;margin:0;padding:12px;background:#111;color:#eee}
h1{font-size:18px} video,#out{max-width:96vw;width:100%;border-radius:8px;background:#000}
video.hidden,#out.hidden{display:none}
.bar{background:#333;height:8px;border-radius:4px;margin:8px auto;max-width:400px}
.bar>i{display:block;height:100%;width:0;background:#5d9;border-radius:4px;transition:width .15s}
button{font-size:16px;padding:12px 24px;border:none;border-radius:8px;background:#2a7;color:#fff;cursor:pointer;margin:6px}
button:disabled{background:#555;cursor:default}
.mode{font-size:14px;color:#9cf;margin:6px}
input{font-size:14px;padding:8px;border-radius:6px;border:1px solid #555;background:#222;color:#eee;width:80%;max-width:320px}
#lib{font-size:11px;color:#666} #info{min-height:40px;font-size:15px}
.hint{font-size:12px;color:#888}
</style></head>
<body>
<h1>qrcd 手机中继</h1>
<div class="mode" id="mode">① 扫码：对准发送端屏幕上的二维码</div>
<div><input id="recvurl" placeholder="接收端地址（留空=本机）" value=""></div>
<video id="cam" autoplay playsinline muted hidden></video>
<canvas id="out"></canvas>
<div class="bar"><i id="prog"></i></div>
<div id="info">点「开始扫描」对准发送端二维码</div>
<button id="scanbtn">开始扫描</button>
<button id="sendbtn" disabled>发送到 PC</button>
<button id="clear">清空</button>
<div class="hint" id="hint"></div>
<div id="lib">QR 解码库加载中…</div>
<script src="/jsQR.js"></script>
<script>
const $=id=>document.getElementById(id);
const video=$('cam'),out=$('out'),cx=out.getContext('2d');
// 默认转发目标 = 本页所在服务器（即打开此页的接收端 PC）
let recvUrl=location.origin+'/api/ingest';
let seen=new Set(),scanning=false,t=null,buf=[],scanTicks=0,lastDecodeTick=0;
function waitLib(){return typeof jsQR!=='undefined'?Promise.resolve():new Promise(r=>setTimeout(()=>waitLib().then(r),200))}
waitLib().then(()=>{$('lib').textContent=typeof jsQR==='undefined'?'⚠ jsQR 加载失败':'QR 解码库就绪 ✓'});
// 把 jsQR 解出的字符串还原为原始字节（与发送端 string(data) 入码对称，每 char→1 字节）
function decStrToBytes(s){const a=new Uint8Array(s.length);for(let i=0;i<s.length;i++)a[i]=s.charCodeAt(i)&0xFF;return a}
async function postOne(bytes){
  const r=await fetch(recvUrl,{method:'POST',headers:{'Content-Type':'application/octet-stream'},body:bytes});
  return await r.json();
}
async function startScan(){
  try{
    const stream=await navigator.mediaDevices.getUserMedia({video:{facingMode:'environment'}});
    video.srcObject=stream;await video.play();scanning=true;scanTicks=0;lastDecodeTick=0;lastTick=0;
    $('scanbtn').textContent='停止扫描';
    $('info').textContent='扫描中… 对准发送端二维码（让二维码占满大部分画面）';
    $('hint').textContent='';
    loop();
  }catch(e){$('info').textContent='摄像头错误: '+e.message}
}
function stopScan(){
  scanning=false;
  if(video.srcObject){video.srcObject.getTracks().forEach(tr=>tr.stop());video.srcObject=null}
  $('scanbtn').textContent='开始扫描';
  if(buf.length)$('info').textContent='已停止，共捕获 '+buf.length+' 块，点「发送到 PC」上传';
  else $('info').textContent='已停止，未捕获到任何帧';
}
function flashBox(L){
  cx.strokeStyle='#5d9';cx.lineWidth=8;cx.beginPath();
  cx.moveTo(L.topLeftCorner.x,L.topLeftCorner.y);cx.lineTo(L.topRightCorner.x,L.topRightCorner.y);
  cx.lineTo(L.bottomRightCorner.x,L.bottomRightCorner.y);cx.lineTo(L.bottomLeftCorner.x,L.bottomLeftCorner.y);cx.closePath();cx.stroke();
  // 不再隐藏视频预览，避免扫描中断；短暂高亮后清掉边框
  setTimeout(()=>{if(scanning){cx.clearRect(0,0,out.width,out.height);cx.drawImage(video,0,0,out.width,out.height)}},150);
}
let lastTick=0;
function loop(){
  if(!scanning)return;
  // 节流到约 10fps：jsQR 解码较重，每帧都跑会拖垮手机并冻结视频流
  const now=Date.now();
  if(now-lastTick<90){requestAnimationFrame(loop);return}
  lastTick=now;
  if(video.readyState>=2&&video.videoWidth>0){
    // 缩小到处理画布（≤640px），jsQR 在较小图像上更快更稳，高分辨率原图反而易失败
    const scale=Math.min(1,640/Math.max(video.videoWidth,video.videoHeight));
    out.width=Math.round(video.videoWidth*scale);out.height=Math.round(video.videoHeight*scale);
    cx.drawImage(video,0,0,out.width,out.height);
    const img=cx.getImageData(0,0,out.width,out.height);
    const r=jsQR(img.data,img.width,img.height,{inversionAttempts:'attemptBoth'});
    scanTicks++;
    if(r&&r.data){
      const key=r.data.length+':'+r.data.slice(0,48);
      if(!seen.has(key)){
        seen.add(key);lastDecodeTick=scanTicks;
        buf.push(decStrToBytes(r.data));
        $('info').textContent='✓ 已捕获 '+buf.length+' 块'+(buf.length>=2?'（可继续扫，或点「发送到 PC」）':'');
        $('sendbtn').disabled=false;
        flashBox(r.location);
      }
    }
    // 长时间未识别到二维码时给出提示
    if(scanTicks-lastDecodeTick>30 && buf.length===0){
      $('hint').textContent='未识别到二维码，请调整距离/角度/光线，或让二维码占满更多画面';
    }else if(buf.length>0){
      $('hint').textContent='';
    }
  }
  requestAnimationFrame(loop);
}
async function sendAll(){
  if(buf.length===0){$('info').textContent='没有可发送的帧，先扫描';return}
  $('sendbtn').disabled=true;$('scanbtn').disabled=true;
  $('info').textContent='发送中… 0/'+buf.length;
  let sent=0,total=0,done=false,lastErr='';
  for(let i=0;i<buf.length;i++){
    try{
      const j=await postOne(buf[i]);
      if(j&&j.total)total=j.total;
      if(j&&j.count)sent=j.count;
      if(j&&j.done)done=true;
      $('prog').style.width=Math.min(100,sent/Math.max(1,total||buf.length)*100)+'%';
      $('info').textContent='发送中 '+(i+1)+'/'+buf.length+'（接收端已收 '+sent+'/'+(total||'?')+'）';
      if(done){$('info').textContent='✓ 上传完成，接收端已还原 ('+sent+'/'+total+')';break}
    }catch(e){lastErr=e.message;$('info').textContent='发送失败: '+e.message+'（已发 '+(i+1)+'/'+buf.length+'），检查接收端地址后重试';break}
  }
  $('scanbtn').disabled=false;
  if(!done&&!lastErr)$('info').textContent='发送完毕 ('+buf.length+' 块已上传)；接收端进度 '+sent+'/'+(total||'?');
  if(!done)$('sendbtn').disabled=false; // 允许重试/补发
}
$('recvurl').onchange=e=>{const v=e.target.value.trim().replace(/\/$/,'');if(v)recvUrl=v+'/api/ingest'};
$('clear').onclick=()=>{seen.clear();buf.length=0;$('prog').style.width=0;$('sendbtn').disabled=true;$('info').textContent='已清空，重新扫描';$('hint').textContent=''};
$('scanbtn').onclick=()=>{scanning?stopScan():startScan()};
$('sendbtn').onclick=sendAll;
</script></body></html>`

// receiverPage 网络接收端：无需摄像头。手机中继把扫到的 QR 帧字节 POST 到 /api/ingest，
// 服务端用 receive.Processor 重组；本页轮询 /api/recv/status 显示进度，完成后下载/提示落盘路径。
const receiverPage = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>qrcd 接收端</title>
<style>body{font-family:system-ui,sans-serif;text-align:center;margin:0;padding:20px;background:#111;color:#eee}
h1{font-size:20px} .hint{color:#9cf;font-size:14px;margin:12px}
.bar{background:#333;height:10px;border-radius:5px;margin:16px auto;max-width:420px}
.bar>i{display:block;height:100%;width:0;background:#5d9;border-radius:5px;transition:width .2s}
button{font-size:16px;padding:12px 24px;border:none;border-radius:8px;background:#2a7;color:#fff;cursor:pointer;margin:6px}
#info{min-height:48px;font-size:15px} .path{font-family:monospace;background:#222;padding:8px;border-radius:6px;display:inline-block;color:#5d9}
</style></head>
<body>
<h1>qrcd 接收端</h1>
<div class="hint">本机已就绪，等待手机中继上传帧。<br>在手机上打开 <b>本机IP:8080/relay</b> 扫发送端二维码即可。</div>
<div class="bar"><i id="prog"></i></div>
<div id="info">等待上传…</div>
<button id="save" hidden>下载文件</button>
<button id="reset">重置（接收新文件）</button>
<script>
const $=id=>document.getElementById(id);
let timer=setInterval(poll,500);
async function poll(){
  try{
    const r=await fetch('/api/recv/status');const j=await r.json();
    if(!j.ready){$('info').textContent='等待上传…';return}
    if(j.total>0)$('prog').style.width=Math.min(100,j.count/Math.max(1,j.total)*100)+'%';
    if(j.done&&j.name){
      $('info').innerHTML='✓ 还原成功：<b>'+j.name+'</b> ('+j.size+'B)<br>已落盘：<span class="path">'+j.path+'</span><br>SHA-256: '+j.sha256.slice(0,16)+'…';
      $('save').hidden=false;$('save').onclick=()=>location.href='/api/recv/file';
    }else{
      $('info').textContent='已收 '+(j.count||0)+'/'+(j.total||'?')+' 块';
    }
  }catch(e){$('info').textContent='查询失败: '+e.message}
}
$('reset').onclick=()=>{fetch('/api/recv/status').then(()=>{$('prog').style.width=0;$('info').textContent='等待上传…';$('save').hidden=true})};
</script></body></html>`
