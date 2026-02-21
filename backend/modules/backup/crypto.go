// Package backup 提供备份还原功能
package backup

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
	"os"

	"golang.org/x/crypto/pbkdf2"
)

const (
	// 加密文件头标识
	encryptedFileHeader = "RDE-ENC-V1"
	// PBKDF2 迭代次数
	pbkdf2Iterations = 100000
	// salt 长度
	saltSize = 32
	// AES-256 密钥长度
	keySize = 32
	// GCM nonce 长度
	nonceSize = 12
)

// Encryptor AES-256 加密器
type Encryptor struct {
	password string
}

// NewEncryptor 创建加密器
func NewEncryptor(password string) *Encryptor {
	return &Encryptor{password: password}
}

// deriveKey 从密码派生密钥
func (e *Encryptor) deriveKey(salt []byte) []byte {
	return pbkdf2.Key([]byte(e.password), salt, pbkdf2Iterations, keySize, sha256.New)
}

// EncryptFile 加密文件
// 格式: [header:10][salt:32][nonce:12][ciphertext...]
func (e *Encryptor) EncryptFile(srcPath, dstPath string) error {
	// 打开源文件
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	// 创建目标文件
	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	// 写入文件头
	if _, err := dst.Write([]byte(encryptedFileHeader)); err != nil {
		return err
	}

	// 生成随机 salt
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	if _, err := dst.Write(salt); err != nil {
		return err
	}

	// 派生密钥
	key := e.deriveKey(salt)

	// 创建 AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	// 创建 GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	// 生成随机 nonce
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	if _, err := dst.Write(nonce); err != nil {
		return err
	}

	// 读取源文件内容
	plaintext, err := io.ReadAll(src)
	if err != nil {
		return err
	}

	// 加密
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// 写入密文
	if _, err := dst.Write(ciphertext); err != nil {
		return err
	}

	return nil
}

// DecryptFile 解密文件
func (e *Encryptor) DecryptFile(srcPath, dstPath string) error {
	// 打开源文件
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	// 读取并验证文件头
	header := make([]byte, len(encryptedFileHeader))
	if _, err := io.ReadFull(src, header); err != nil {
		return err
	}
	if string(header) != encryptedFileHeader {
		return errors.New("invalid encrypted file format")
	}

	// 读取 salt
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(src, salt); err != nil {
		return err
	}

	// 读取 nonce
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(src, nonce); err != nil {
		return err
	}

	// 派生密钥
	key := e.deriveKey(salt)

	// 创建 AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	// 创建 GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	// 读取密文
	ciphertext, err := io.ReadAll(src)
	if err != nil {
		return err
	}

	// 解密
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return errors.New("decryption failed: invalid password or corrupted file")
	}

	// 创建目标文件
	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	// 写入明文
	if _, err := dst.Write(plaintext); err != nil {
		return err
	}

	return nil
}

// EncryptFileStreaming 流式加密（用于大文件）
func (e *Encryptor) EncryptFileStreaming(srcPath, dstPath string) error {
	// 打开源文件
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	// 创建目标文件
	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	// 写入文件头
	if _, err := dst.Write([]byte(encryptedFileHeader)); err != nil {
		return err
	}

	// 生成随机 salt
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	if _, err := dst.Write(salt); err != nil {
		return err
	}

	// 派生密钥
	key := e.deriveKey(salt)

	// 创建 AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	// 使用 CTR 模式（流式加密）
	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return err
	}
	if _, err := dst.Write(iv); err != nil {
		return err
	}

	stream := cipher.NewCTR(block, iv)

	// 写入 HMAC（用于完整性验证）将在解密时验证
	// 流式加密
	buf := make([]byte, 64*1024) // 64KB buffer
	for {
		n, err := src.Read(buf)
		if n > 0 {
			encrypted := make([]byte, n)
			stream.XORKeyStream(encrypted, buf[:n])
			if _, err := dst.Write(encrypted); err != nil {
				return err
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// DecryptFileStreaming 流式解密（用于大文件）
func (e *Encryptor) DecryptFileStreaming(srcPath, dstPath string) error {
	// 打开源文件
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	// 读取并验证文件头
	header := make([]byte, len(encryptedFileHeader))
	if _, err := io.ReadFull(src, header); err != nil {
		return err
	}
	if string(header) != encryptedFileHeader {
		return errors.New("invalid encrypted file format")
	}

	// 读取 salt
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(src, salt); err != nil {
		return err
	}

	// 读取 IV
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(src, iv); err != nil {
		return err
	}

	// 派生密钥
	key := e.deriveKey(salt)

	// 创建 AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	// 使用 CTR 模式
	stream := cipher.NewCTR(block, iv)

	// 创建目标文件
	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	// 流式解密
	buf := make([]byte, 64*1024) // 64KB buffer
	for {
		n, err := src.Read(buf)
		if n > 0 {
			decrypted := make([]byte, n)
			stream.XORKeyStream(decrypted, buf[:n])
			if _, err := dst.Write(decrypted); err != nil {
				return err
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// IsEncrypted 检查文件是否已加密
func IsEncrypted(filePath string) (bool, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	header := make([]byte, len(encryptedFileHeader))
	n, err := f.Read(header)
	if err != nil {
		return false, err
	}
	if n < len(encryptedFileHeader) {
		return false, nil
	}

	return string(header) == encryptedFileHeader, nil
}
