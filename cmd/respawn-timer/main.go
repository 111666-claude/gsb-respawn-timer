// Command respawn-timer 跑复活队列样例。
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
	sample := flags.String("sample", "boundary", "boundary / duplicate / teamcap / cost")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	switch *sample {
	case "boundary":
		// 冷却 10000ms，玩家在 5000ms 死亡，15000ms 正好够冷却。
		queue := respawn.NewQueue(10000, 2)
		queue.Enqueue(respawn.Player{Name: "a", Team: "red", DeadAt: 5000})
		chosen, ok := queue.Select(15000)
		if !ok {
			fmt.Fprintln(stdout, "ready=false")
			return 0
		}
		fmt.Fprintf(stdout, "ready=true name=%s\n", chosen.Name)
	case "duplicate":
		// 同一个玩家三次阵亡上报。
		queue := respawn.NewQueue(10000, 2)
		for _, deadAt := range []int64{1000, 2000, 3000} {
			queue.Enqueue(respawn.Player{Name: "a", Team: "red", DeadAt: deadAt})
		}
		chosen, ok := queue.Select(60000)
		if !ok {
			fmt.Fprintf(stdout, "size=%d ready=false\n", queue.Size())
			return 0
		}
		fmt.Fprintf(stdout, "size=%d deadat=%d\n", queue.Size(), chosen.DeadAt)
	case "teamcap":
		// 红队已经站着两个人（上限 2），红队 a 比蓝队 b 死得早。
		queue := respawn.NewQueue(10000, 2)
		queue.Occupy("red", 2)
		queue.Enqueue(respawn.Player{Name: "a", Team: "red", DeadAt: 0})
		queue.Enqueue(respawn.Player{Name: "b", Team: "blue", DeadAt: 1000})
		chosen, ok := queue.Select(60000)
		if !ok {
			fmt.Fprintln(stdout, "picked=none")
			return 0
		}
		fmt.Fprintf(stdout, "picked=%s\n", chosen.Name)
	case "cost":
		// 两千人在队，连续查五千次。
		queue := respawn.NewQueue(1000, 100)
		for index := 0; index < 2000; index++ {
			queue.Enqueue(respawn.Player{Name: fmt.Sprintf("p-%d", index), Team: "red", DeadAt: int64(index)})
		}
		for query := 0; query < 5000; query++ {
			queue.Select(0)
		}
		scanned := queue.Scanned()
		if scanned <= 50000 {
			fmt.Fprintf(stdout, "scanned<=50000 size=%d\n", queue.Size())
		} else {
			fmt.Fprintf(stdout, "scanned=%d size=%d\n", scanned, queue.Size())
		}
	default:
		fmt.Fprintln(stderr, "需要 --sample boundary|duplicate|teamcap|cost")
		return 2
	}
	return 0
}

func main() {
	os.Exit(Run(os.Args[1:], os.Stdout, os.Stderr))
}
