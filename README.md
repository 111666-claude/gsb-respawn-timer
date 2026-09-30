# respawn-timer

复活队列：冷却台账、队伍在场人数上限与复活顺序选取，只用 Go 标准库。

```
go test ./...
go run ./cmd/respawn-timer --sample boundary
go run ./cmd/respawn-timer --sample duplicate
go run ./cmd/respawn-timer --sample teamcap
go run ./cmd/respawn-timer --sample cost
```

## 口径（README 为准）

- **冷却含边界**：`ReadyAt = DeadAt + CooldownMs`，`now >= ReadyAt` 就能复活。
- **同一玩家只留一条记录**：同一个 `Name` 重复阵亡上报按最新 `DeadAt` 合并，
  冷却与顺序都按最新那条算。
- **队伍上限**：一支队伍的在场人数达到 `teamCap` 时该队的候选必须跳过，
  复活成功后调用 `Release` 把名额还回去。
- **选取顺序**：在可复活的候选里按 `ReadyAt` 升序、同刻按名字升序挑一个，
  选中的人从队列移除；没有可复活的人返回 `false`。
- **内存观测**：`Size()` 只统计还在队等待的玩家；`Scanned()` 是选取时看过的记录数，
  必须摊还 O(log n)，不许随在队人数线性增长。
- **规模**：50 万玩家、每秒 20 万次选取，内存 O(在队玩家数)。

## 输出契约（不改格式）

```
ready=true name=a
size=0 deadat=3000
picked=b
scanned<=50000 size=2000
```

## 目录

```
respawn.go              复活队列
cmd/respawn-timer       命令行入口
respawn_test.go         go test 用例
```
