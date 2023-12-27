// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"errors"

	"math/rand"
	"strings"
)

/**
 * 对称加密
 */

const (
	BlockSize = 32
	randomLen = 16 // 随机值长度
)

/**
 * 对要发送的消息进行AES-CBC加密
 */
var encodingAesKey = "DflXyFwTmaoGmbDkVj8uD62XGb01pkJn"

func EncryptMsg(data []byte) ([]byte, error) {
	// 获得16位随机字符串，填充到明文之前
	replyMsgBytes, err := GetRandomBytes(randomLen)
	if err != nil {
		return nil, err
	}
	replyMsgBytes = append(replyMsgBytes, data...)
	// 取秘钥的前16位作为AES-CBC加密的向量iv
	iv := []byte(Substr(encodingAesKey, 0, 16))

	// 根据 key 生成密文
	block, err := aes.NewCipher([]byte(encodingAesKey))
	if err != nil {
		logger.Warningf("[encrypt] aes-cbc encrypt data failed,error:%v", err)
		return nil, err
	}
	encodeBytes := replyMsgBytes
	encodeBytes = PKCS7Encode(encodeBytes, BlockSize)
	blockMode := cipher.NewCBCEncrypter(block, iv)
	crypted := make([]byte, len(encodeBytes))
	if len(crypted)%block.BlockSize() != 0 {
		return nil, errors.New("encrypt failed, input not full blocks")
	}
	blockMode.CryptBlocks(crypted, encodeBytes)
	return crypted, nil
}

// PKCS7Encode 对需要加密的明文进行填充补位 * @param text 需要进行填充补位操作的明文 * @return 补齐明文字符串
func PKCS7Encode(text []byte, blockSize int) []byte {
	textLen := len(text)
	padding := blockSize - (textLen % blockSize)
	if padding == 0 {
		padding = blockSize
	}

	paddingByte := bytes.Repeat([]byte(string(rune(padding))), padding)

	return append(text, paddingByte...)
}

// GetRandomBytes 根据需要长度,生成随机字符
func GetRandomBytes(length uint32) ([]byte, error) {
	res := make([]byte, length)
	_, err := rand.Read(res)
	if err != nil {
		logger.Warning(err)
		return nil, err
	}
	return res, nil
}

// Substr 截取字符串 start 起点下标 length 需要截取的长度
func Substr(str string, start int, length int) string {
	if start < 0 {
		start = 0
	}
	if length < 0 {
		length = 0
	}

	strSlice := strings.Split(str, "")
	strSliceLen := len(strSlice)

	end := start + length

	if end > strSliceLen {
		end = strSliceLen
	}
	if start > strSliceLen {
		start = strSliceLen
	}

	return strings.Join(strSlice[start:end], "")
}
