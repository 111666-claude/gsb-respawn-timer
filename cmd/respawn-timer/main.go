// Command respawn-timer 跑复活样例。
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"example.com/respawn"
)

// Run 执行一次命令行调用，返回退出码。
func Run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("respawn-timer", flag.ContinueOnError)
	flags.SetOutput(stderr)
	sample := flags.String("sample", "cooldown", "cooldown / camped / unsafe")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	manager := &respawn.Manager{CooldownMs: 10000}
	var point respawn.SpawnPoint
	var err error
	switch *sample {
	case "cooldown":
		manager.Die(0, 0, 1000)
		point, err = manager.Respawn(3000, []respawn.SpawnPoint{{Name: "a", X: 1}})
	case "camped":
		manager.Die(0, 0, 1000)
		point, err = manager.Respawn(20000, []respawn.SpawnPoint{
			{Name: "a", X: 1, Enemies: 2},
			{Name: "b", X: 7},
		})
	case "unsafe":
		manager.Die(0, 0, 1000)
		point, err = manager.Respawn(20000, []respawn.SpawnPoint{
			{Name: "a", X: 1, Enemies: 2},
			{Name: "b", X: 7, Enemies: 1},
		})
	default:
		fmt.Fprintln(stderr, "需要 --sample cooldown|camped|unsafe")
		return 2
	}
	if err != nil {
		fmt.Fprintf(stdout, "error=%s\n", errText(err))
		return 0
	}
	fmt.Fprintf(stdout, "spawn=%s\n", point.Name)
	return 0
}

func errText(err error) string {
	switch err {
	case respawn.ErrCooldown:
		return "cooldown"
	case respawn.ErrNoSafe:
		return "unsafe"
	case respawn.ErrNotDead:
		return "notdead"
	}
	return "other"
}

func main() {
	os.Exit(Run(os.Args[1:], os.Stdout, os.Stderr))
}
