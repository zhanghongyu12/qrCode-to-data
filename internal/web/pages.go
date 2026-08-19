package web

// scanTestPage 静态 QR 自检页：把第 0 帧 PNG 画到 canvas，直接用 jsQR 解码，
// 不经过摄像头。用于定位是"QR 内容 jsQR 解不出"还是"摄像头抓取"的问题。
const scanTestPage = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>qrcd 扫码自检</title>
<style>body{font-family:system-ui,sans-serif;text-align:center;margin:0;padding:16px;background:#111;color:#eee}
#qr{image-rendering:pixelated;background:#fff;width:300px;height:300px}
pre{background:#222;padding:8px;border-radius:6px;text-align:left;max-width:600px;margin:8px auto;overflow:auto;font-size:11px;color:#9cf}</style></head>
<body>
<h1>qrcd 扫码自检</h1>
<p>用 jsQR 直接解码第 0 帧 PNG（不经过摄像头）。</p>
<button id="run">开始自检</button>
<canvas id="qr" width="300" height="300"></canvas>
<div id="info">点按钮开始</div>
<pre id="raw"></pre>
<script src="/jsQR.js"></script>
<script>
const $=id=>document.getElementById(id);
$('run').onclick=async()=>{
  $('info').textContent='jsQR: '+(typeof jsQR!=='undefined'?'已加载':'未加载!');
  if(typeof jsQR==='undefined')return;
  const img=new Image();
  img.crossOrigin='anonymous';
  img.onload=()=>{
    const c=$('qr'),x=c.getContext('2d');
    c.width=img.width;c.height=img.height;x.drawImage(img,0,0);
    const d=x.getImageData(0,0,c.width,c.height);
    const r=jsQR(d.data,d.width,d.height,{inversionAttempts:'attemptBoth'});
    if(r&&r.data){
      $('info').textContent='✓ jsQR 解码成功，长度='+r.data.length+' 字节';
      $('raw').textContent='前80字节(charCode): '+[...r.data.slice(0,80)].map(ch=>ch.charCodeAt(0)&0xFF).join(' ')
        +'\n\n原始字符串前80字符: '+r.data.slice(0,80);
    }else{
      $('info').textContent='✗ jsQR 解码失败（返回 null）—— QR 内容 jsQR 解不出';
      $('raw').textContent='画布尺寸='+c.width+'x'+c.height;
    }
  };
  img.onerror=()=>$('info').textContent='加载 PNG 失败（请先在发送端编码生成帧）';
  img.src='/api/frame/0?_='+Date.now();
};
</script></body></html>`

// indexPage 首页：三端入口 + 手机 App 下载。
const indexPage = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>qrcd 三端</title>
<style>body{font-family:system-ui,sans-serif;max-width:680px;margin:40px auto;padding:0 20px;color:#222}
a{display:block;padding:20px;margin:12px 0;border:2px solid #2a7;border-radius:12px;text-decoration:none;color:#2a7;font-size:18px}
a:hover{background:#2a7;color:#fff}
small{color:#888}
.appdl{border-color:#25a;background:#25a;color:#fff}
.appdl:hover{background:#138}
.scanhint{font-size:13px;color:#555;margin-top:8px}
#appqr{margin:12px auto;background:#fff;padding:10px;border-radius:8px;display:none;image-rendering:pixelated}
</style></head>
<body>
<h1>qrcd · 二维码数据传输</h1>
<p>三端协作，纯光学（屏幕二维码 ↔ 摄像头），全程不联网：</p>
<a class="appdl" href="/dl/app.apk">📱 下载手机中继 App（Android）</a>
<div class="scanhint">手机扫下面的二维码可直接打开下载页（手机浏览器访问本地址）：</div>
<img id="appqr" alt="App下载二维码">
<button id="showqr" onclick="showAppQR()">显示下载二维码</button>
<hr>
<a href="/sender">① 发送端 —— 选文件，屏幕播放二维码</a>
<a href="/relay">② 手机中继（网页版） —— 扫码后转发给接收端</a>
<a href="/receiver">③ 接收端 —— 接收手机上传的帧，重组并下载</a>
<hr><small>发送端电脑 →（手机扫码）→ 手机 →（网络）→ 接收端电脑。接收端无需摄像头。<br>
推荐用「手机中继 App」：原生扫码识别率远高于网页 jsQR，无需 HTTPS/证书。</small>
<script>
function showAppQR(){
  const host=location.hostname+(location.port?':'+location.port:'');
  const url='http://'+host+'/dl/app.apk';
  const img=document.getElementById('appqr');
  img.src='/api/qrcode?d='+encodeURIComponent(url);
  img.style.display='block';
  document.getElementById('showqr').hidden=true;
}
</script>
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
h1{font-size:18px}
#camwrap{position:relative;max-width:96vw;margin:8px auto}
#cam{width:100%;border-radius:8px;background:#000;display:block}
#overlay{position:absolute;left:0;top:0;width:100%;height:100%;pointer-events:none}
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
<div id="camwrap">
<video id="cam" autoplay playsinline muted></video>
<canvas id="overlay"></canvas>
</div>
<canvas id="out" hidden></canvas>
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
const video=$('cam'),out=$('out'),cx=out.getContext('2d'),ov=$('overlay'),ox=ov.getContext('2d');
// 默认转发目标 = 本页所在服务器（即打开此页的接收端 PC）
let recvUrl=location.origin+'/api/ingest';
let seen=new Set(),scanning=false,buf=[],scanTicks=0,lastDecodeTick=0;
// 优先用浏览器原生 BarcodeDetector（调用手机系统条码引擎，识别率/速度远超 jsQR）
let detector=null,useNative=false,nativeTried=false;
async function initDetector(){
  if(typeof BarcodeDetector==='undefined')return false;
  try{
    const d=new BarcodeDetector({formats:['qr_code']});
    const sup=await BarcodeDetector.getSupportedFormats();
    if(!sup||sup.indexOf('qr_code')<0)return false;
    detector=d;return true;
  }catch(e){return false}
}
function showLib(){
  if(useNative)$('lib').textContent='原生扫码引擎就绪 ✓（系统级）';
  else $('lib').textContent=typeof jsQR!=='undefined'?'QR 解码库就绪 ✓（jsQR）':'⚠ jsQR 加载失败';
}
initDetector().then(ok=>{useNative=ok;showLib()});
function waitLib(){return typeof jsQR!=='undefined'?Promise.resolve():new Promise(r=>setTimeout(()=>waitLib().then(r),200))}
waitLib().then(showLib);
// QR 内容是帧字节的 base64（纯可打印 ASCII，jsQR 解码可靠）。
// 解出后 atob 还原二进制串 → 每字符 1 字节。
function decStrToBytes(s){
  const bin=atob(s);
  const a=new Uint8Array(bin.length);
  for(let i=0;i<bin.length;i++)a[i]=bin.charCodeAt(i)&0xFF;
  return a;
}
async function postOne(bytes){
  const r=await fetch(recvUrl,{method:'POST',headers:{'Content-Type':'application/octet-stream'},body:bytes});
  return await r.json();
}
async function startScan(){
  try{
    const stream=await navigator.mediaDevices.getUserMedia({video:{facingMode:'environment'}});
    video.srcObject=stream;await video.play();
    // 等视频尺寸就绪后，给 overlay 画布对齐到视频显示分辨率
    if(!video.videoWidth){await new Promise(r=>video.onloadedmetadata=r);await video.play()}
    ov.width=video.clientWidth;ov.height=video.clientHeight;
    scanning=true;scanTicks=0;lastDecodeTick=0;
    $('scanbtn').textContent='停止扫描';
    $('info').textContent='扫描中… 对准发送端二维码（让二维码占满大部分画面）';
    $('hint').textContent='';
    tick();
  }catch(e){$('info').textContent='摄像头错误: '+e.message}
}
function stopScan(){
  scanning=false;
  if(video.srcObject){video.srcObject.getTracks().forEach(tr=>tr.stop());video.srcObject=null}
  $('scanbtn').textContent='开始扫描';
  if(buf.length)$('info').textContent='已停止，共捕获 '+buf.length+' 块，点「发送到 PC」上传';
  else $('info').textContent='已停止，未捕获到任何帧';
}
// overlay 坐标系 = 视频显示像素；box 来自视频原始坐标，需按显示比例换算
function drawBox(b){
  const sx=ov.clientWidth/video.videoWidth, sy=ov.clientHeight/video.videoHeight;
  ox.clearRect(0,0,ov.width,ov.height);
  ox.strokeStyle='#5d9';ox.lineWidth=4;
  ox.strokeRect(b.x*sx,b.y*sy,b.width*sx,b.height*sy);
}
function clearBox(){ox.clearRect(0,0,ov.width,ov.height)}
async function tick(){
  if(!scanning)return;
  let text=null,box=null;
  if(video.readyState>=2&&video.videoWidth>0){
    // 处理画布 ≤640px（供 jsQR 回退），overlay 与视频显示尺寸对齐
    const scale=Math.min(1,640/Math.max(video.videoWidth,video.videoHeight));
    out.width=Math.round(video.videoWidth*scale);out.height=Math.round(video.videoHeight*scale);
    cx.drawImage(video,0,0,out.width,out.height);
    scanTicks++;
    if(useNative&&detector){
      try{
        nativeTried=true;
        const codes=await detector.detect(video);
        if(codes&&codes.length){
          text=codes[0].rawValue;
          const b=codes[0].boundingBox;
          box={x:b.x,y:b.y,width:b.width,height:b.height};
        }
      }catch(e){}
    }
    if(text===null&&typeof jsQR!=='undefined'){
      const img=cx.getImageData(0,0,out.width,out.height);
      const r=jsQR(img.data,img.width,img.height,{inversionAttempts:'attemptBoth'});
      if(r&&r.data){
        text=r.data;
        const L=r.location;
        // jsQR 坐标在处理画布(scale)上，换算回视频原始坐标
        box={x:L.topLeftCorner.x/scale,y:L.topLeftCorner.y/scale,width:(L.topRightCorner.x-L.topLeftCorner.x)/scale,height:(L.bottomLeftCorner.y-L.topLeftCorner.y)/scale};
      }
    }
    if(text){
      const key=text.length+':'+text.slice(0,48);
      if(!seen.has(key)){
        seen.add(key);lastDecodeTick=scanTicks;
        buf.push(decStrToBytes(text));
        $('info').textContent='✓ 已捕获 '+buf.length+' 块'+(buf.length>=2?'（可继续扫，或点「发送到 PC」）':'');
        $('sendbtn').disabled=false;
      }
      if(box)drawBox(box);
    }else{clearBox()}
    if(scanTicks-lastDecodeTick>30 && buf.length===0){
      $('hint').textContent='未识别到二维码，请调整距离/角度/光线，或让二维码占满更多画面'
        +(nativeTried?'':'');
    }else if(buf.length>0){$('hint').textContent=''}
  }
  setTimeout(tick,useNative?120:100); // 约 8-10fps，顺序执行避免重叠拖垮
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
