package shell

import (
	"fmt"
	"strings"
)

// defaultBlacklist 给出默认的危险命令黑名单。
//
// 设计原则（参考 one_loop.md 5.7）：
//  1. 宁可误拦也不漏放：用户可以手动执行被拦截的命令，Agent 不能自动执行；
//  2. 模式匹配而非精确匹配：使用 strings.Contains 在小写后的命令文本里查找，能覆盖大部分变体；
//  3. 覆盖跨平台常见危险操作：同时包含 Unix（rm -rf /）与 Windows（Remove-Item -Recurse -Force）。
func defaultBlacklist() []string {
	return []string{
		// ====== 类 Unix 危险命令 ======
		"rm -rf /",
		"rm -rf /*",
		"rm -rf ~",
		"mkfs",
		"> /dev/sda",
		"dd if=",
		"shutdown",
		"reboot",
		"init 0",
		"init 6",
		"poweroff",
		"halt",
		"chmod 777",
		"chmod -r 777",
		"sudo su",

		// ====== 版本控制 / 历史破坏 ======
		"git push --force",
		"git push -f",
		"git reset --hard",

		// ====== Windows / PowerShell 危险命令 ======
		"remove-item -recurse -force",
		"remove-item -force -recurse",
		"rmdir /s",
		"del /f /s /q",
		"format-volume",
		"stop-computer",
		"restart-computer",
	}
}

// isBlocked 判断命令是否命中黑名单。
// 同时返回命中的规则文本，便于给模型解释为何被拦截。
func (t *Tool) isBlocked(command string) (string, bool) {
	cmdLower := strings.ToLower(strings.TrimSpace(command))
	for _, pattern := range t.blacklist {
		if strings.Contains(cmdLower, strings.ToLower(pattern)) {
			return fmt.Sprintf("命中黑名单规则 %q", pattern), true
		}
	}
	return "", false
}
