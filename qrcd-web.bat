@echo off
chcp 65001 >nul
cd /d "%~dp0"
title qrcd 二维码传输 - 三端
echo.
echo   qrcd 三端已启动，浏览器即将打开...
echo     发送端(本机选文件):   http://localhost:8080/sender
echo     手机中继(扫码重放):   http://localhost:8080/relay
echo     接收端(扫码还原):    http://localhost:8080/receiver
echo.
echo   手机访问请把 localhost 换成本机 IP(见下方)
ipconfig | findstr /C:"IPv4" /C:"IPv4 地址"
echo.
echo   关闭本窗口即停止服务。
echo.
start "" http://localhost:8080/
qrcd.exe web --addr :8080
pause
