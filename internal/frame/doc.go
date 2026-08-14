// Package frame 实现二维码帧协议。
// 包含 32 字节帧头（magic/version/type/flags/transfer_id/seq/len）+ payload + 4 字节 CRC32，
// 支持组帧、拆帧、元数据帧 JSON 解析、seq 去重、transfer_id 会话区分。
package frame