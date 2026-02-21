package file

import (
	"bufio"
	"io"
	"os"
)

// ChunkReader 分块读取器
type ChunkReader struct {
	file      *os.File
	chunkSize int
}

// NewChunkReader 创建分块读取器
func NewChunkReader(path string, chunkSize int) (*ChunkReader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &ChunkReader{
		file:      file,
		chunkSize: chunkSize,
	}, nil
}

// Read 读取一个块
func (r *ChunkReader) Read() ([]byte, error) {
	buf := make([]byte, r.chunkSize)
	n, err := r.file.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

// Close 关闭读取器
func (r *ChunkReader) Close() error {
	return r.file.Close()
}

// ReadChunks 分块读取文件并处理
func ReadChunks(path string, chunkSize int, handler func(chunk []byte) error) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	buf := make([]byte, chunkSize)
	for {
		n, err := file.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if err := handler(buf[:n]); err != nil {
			return err
		}
	}
	return nil
}

// LineReader 行读取器
type LineReader struct {
	file    *os.File
	scanner *bufio.Scanner
}

// NewLineReader 创建行读取器
func NewLineReader(path string) (*LineReader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &LineReader{
		file:    file,
		scanner: bufio.NewScanner(file),
	}, nil
}

// ReadLine 读取一行
func (r *LineReader) ReadLine() (string, bool) {
	if r.scanner.Scan() {
		return r.scanner.Text(), true
	}
	return "", false
}

// Err 获取错误
func (r *LineReader) Err() error {
	return r.scanner.Err()
}

// Close 关闭读取器
func (r *LineReader) Close() error {
	return r.file.Close()
}

// ReadLinesWithCallback 按行读取文件并回调处理
func ReadLinesWithCallback(path string, handler func(line string) error) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if err := handler(scanner.Text()); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// TailFile 读取文件末尾 n 行
func TailFile(path string, n int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > n {
			lines = lines[1:]
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}

// HeadFile 读取文件开头 n 行
func HeadFile(path string, n int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() && len(lines) < n {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}
