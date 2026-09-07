# 双击此文件 → UAC 弹窗点「是」→ 自动放行 qrcd 的 8080/8443 入站。
# 若已是管理员则直接添加。添加完会显示「完成」并 3 秒后自动关窗。
if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Start-Process powershell -Verb RunAs -ArgumentList "-NoProfile -ExecutionPolicy Bypass -File `"$PSCommandPath`""
    exit
}
New-NetFirewallRule -DisplayName "qrcd-8080" -Direction Inbound -Protocol TCP -LocalPort 8080 -Action Allow -Profile Any -ErrorAction SilentlyContinue | Out-Null
New-NetFirewallRule -DisplayName "qrcd-8443" -Direction Inbound -Protocol TCP -LocalPort 8443 -Action Allow -Profile Any -ErrorAction SilentlyContinue | Out-Null
Write-Host ""
Write-Host "==> 防火墙已放行 qrcd 入站：8080(HTTP) + 8443(HTTPS)" -ForegroundColor Green
Write-Host "==> 手机访问：https://10.70.19.160:8443/relay" -ForegroundColor Cyan
Write-Host "==> 首次提示证书不安全 → 点「高级 / 详细信息」→「继续前往」" -ForegroundColor Yellow
Write-Host "==> 确保手机和电脑连同一个 WiFi" -ForegroundColor Yellow
Write-Host ""
Start-Sleep -Seconds 4
