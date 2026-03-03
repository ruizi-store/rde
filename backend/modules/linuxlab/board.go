package linuxlab

import (
	"bufio"
	"regexp"
	"strconv"
	"strings"
)

// 匹配 Makefile 变量赋值: VAR ?= value / VAR := value / VAR = value
var makeVarRegex = regexp.MustCompile(`^\s*([A-Z_][A-Z_0-9]*)\s*[\?:]?=\s*(.*)$`)

// ParseBoardFromContent 从 Makefile 文本内容解析开发板配置
func ParseBoardFromContent(content string, arch string, mach string) *Board {
	vars := parseMakeVars(content)

	board := &Board{
		Arch:      arch,
		Name:      mach,
		FullPath:  arch + "/" + mach,
		CPU:       vars["CPU"],
		MEM:       vars["MEM"],
		Linux:     vars["LINUX"],
		QEMU:      vars["QEMU"],
		UBoot:     vars["UBOOT"],
		Buildroot: vars["BUILDROOT"],
		NetDev:    vars["NETDEV"],
		Serial:    vars["SERIAL"],
		RootDev:   vars["ROOTDEV"],
	}

	if smpStr := vars["SMP"]; smpStr != "" {
		if smp, err := strconv.Atoi(smpStr); err == nil {
			board.SMP = smp
		}
	}

	if a := vars["ARCH"]; a != "" {
		board.Arch = a
	}

	return board
}

// parseMakeVars 从 Makefile 内容中提取变量赋值
func parseMakeVars(content string) map[string]string {
	vars := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		matches := makeVarRegex.FindStringSubmatch(line)
		if len(matches) == 3 {
			key := strings.TrimSpace(matches[1])
			value := strings.TrimSpace(matches[2])
			if idx := strings.Index(value, "#"); idx > 0 {
				value = strings.TrimSpace(value[:idx])
			}
			if _, exists := vars[key]; !exists {
				vars[key] = value
			}
		}
	}
	return vars
}
