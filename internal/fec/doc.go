// Package fec 实现喷泉码编解码接口。
// 基于 gofountain 库，默认 Raptor 码（RFC 5053），LT 码兜底，blockCount=1 时短路。
// 接口契约定义在 docs/04_api.md §3。
package fec