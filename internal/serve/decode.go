package serve

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"bytes"
	"encoding/json"
)

func decodeJSONLine(line []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.UseNumber()
	return dec.Decode(dst)
}

func decodeParams(raw json.RawMessage, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	return dec.Decode(dst)
}

