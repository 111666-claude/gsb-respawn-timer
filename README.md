# respawn-timer

死亡与复活：冷却校验、安全点挑选、状态清理，只用 Go 标准库。

```
go test ./...
go run ./cmd/respawn-timer --sample cooldown
go run ./cmd/respawn-timer --sample camped
go run ./cmd/respawn-timer --sample unsafe
```

## 口径（README 为准）

- **复活要过冷却**：距 `Die` 的时刻不足 `CooldownMs` 时必须拒绝，返回 `ErrCooldown`，
  并且不能改动任何状态。`--sample cooldown`（1000ms 死亡、3000ms 请求复活、冷却 10000ms）
  按口径是 `error=cooldown`，现在是 `spawn=a`。
- **安全点里挑最近的**：`Enemies > 0` 的点（半径 `SafeRadius` 内有敌人）必须整个跳过，
  在剩下的点里挑距死亡坐标最近的，并列时按名字字典序升序。
  `--sample camped`（近点 a 有 2 个敌人、远点 b 干净）按口径是 `spawn=b`，现在是 `spawn=a`。
- **没有安全点要报错**：一个干净点都没有时返回 `ErrNoSafe`，不许返回零值点。
  `--sample unsafe`（两个点都有敌人）按口径是 `error=unsafe`，现在是 `spawn=a`。
- 没有死亡记录时调用复活返回 `ErrNotDead`；复活成功后才清掉死亡状态与死亡时刻。
- 不变量：失败的复活调用不改状态（重复请求的返回值一致）；复活成功后 `Dead()` 为假；
  同一份点列表重复求值结果相同。
- 规模：单次复活在 1 万个候选点里 O(n) 选完，内存 O(n)；不许对整表排序。
- 点列表里的敌人信息是快照，复活期间不重新查询，也不允许把敌人算成半个（有敌即不可用）。

## 现在的行为

- `Respawn` 不看 `CooldownMs`，也不看 `Dead()`，冷却里能连着复活。
- `Respawn` 不看 `Enemies`，被敌人围住的点照样用；没有点时返回零值 `SpawnPoint` 和 `nil`。

## 输出

```
error=cooldown
spawn=b
error=unsafe
```

## 目录

```
respawn.go              复活与安全点
cmd/respawn-timer       命令行入口
respawn_test.go         go test 用例
```
