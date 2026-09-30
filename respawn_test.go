package respawn

import (
	"math/rand"
	"sort"
	"testing"
)

func TestSelectAfterCooldown(t *testing.T) {
	queue := NewQueue(DefaultCooldownMs, 2)
	queue.Enqueue(Player{Name: "a", Team: "red", DeadAt: 0})
	chosen, ok := queue.Select(20000)
	if !ok {
		t.Fatal("冷却走完以后应该有人可以复活")
	}
	if chosen.Name != "a" {
		t.Fatalf("复活的人不对：%+v", chosen)
	}
	if queue.Size() != 0 {
		t.Fatalf("复活之后应该从队列移除：%d", queue.Size())
	}
}

func TestSelectEmptyQueue(t *testing.T) {
	if _, ok := NewQueue(DefaultCooldownMs, 2).Select(1000); ok {
		t.Fatal("空队列不该选出人")
	}
}

func TestSizeCountsDistinctPlayers(t *testing.T) {
	queue := NewQueue(DefaultCooldownMs, 2)
	queue.Enqueue(Player{Name: "a", Team: "red", DeadAt: 0})
	queue.Enqueue(Player{Name: "b", Team: "blue", DeadAt: 100})
	if queue.Size() != 2 {
		t.Fatalf("两个不同玩家应该占两条记录：%d", queue.Size())
	}
}

func TestReleaseFreesTeamSlot(t *testing.T) {
	queue := NewQueue(DefaultCooldownMs, 1)
	queue.Occupy("red", 1)
	queue.Release("red")
	if queue.teamCounts["red"] != 0 {
		t.Fatalf("释放之后队伍名额应该空出来：%d", queue.teamCounts["red"])
	}
}

func TestConstants(t *testing.T) {
	if DefaultCooldownMs != 10000 {
		t.Fatal("冷却常量被改了")
	}
}

// 冷却含边界：now == DeadAt + CooldownMs 就能复活；差 1ms 不行。
func TestCooldownBoundary(t *testing.T) {
	queue := NewQueue(10000, 2)
	queue.Enqueue(Player{Name: "a", Team: "red", DeadAt: 5000})
	if _, ok := queue.Select(14999); ok {
		t.Fatal("冷却差 1ms 时不该复活")
	}
	if _, ok := queue.Select(15000); !ok {
		t.Fatal("now 正好等于 ReadyAt 时必须能复活")
	}
}

// 同一玩家三次阵亡上报：只留一条，冷却与 DeadAt 都按最新那条算。
func TestDuplicateMergesLatest(t *testing.T) {
	queue := NewQueue(10000, 2)
	for _, deadAt := range []int64{1000, 2000, 3000} {
		queue.Enqueue(Player{Name: "a", Team: "red", DeadAt: deadAt})
		if queue.Size() != 1 {
			t.Fatalf("重复阵亡上报后队列里只能有一条记录：%d", queue.Size())
		}
	}
	// 最新记录 ReadyAt=13000；旧记录 11000/12000 不能让他提前起来。
	if _, ok := queue.Select(12999); ok {
		t.Fatal("最新阵亡的冷却没走完时不该复活")
	}
	chosen, ok := queue.Select(13000)
	if !ok {
		t.Fatal("最新阵亡冷却到点时必须能复活")
	}
	if chosen.DeadAt != 3000 {
		t.Fatalf("复活记录必须是最新一次阵亡：deadat=%d", chosen.DeadAt)
	}
	if queue.Size() != 0 {
		t.Fatalf("复活后旧记录也必须消失：%d", queue.Size())
	}
}

// 队伍满员时跳过该队候选，按顺序挑下一队的人；名额释放后被跳过的人还能复活。
func TestTeamCapSkipsFullTeam(t *testing.T) {
	queue := NewQueue(10000, 2)
	queue.Occupy("red", 2)
	queue.Enqueue(Player{Name: "a", Team: "red", DeadAt: 0})
	queue.Enqueue(Player{Name: "b", Team: "blue", DeadAt: 1000})

	chosen, ok := queue.Select(60000)
	if !ok || chosen.Name != "b" {
		t.Fatalf("红队满员时应跳过 a 选 b：%+v %v", chosen, ok)
	}
	if queue.Size() != 1 {
		t.Fatalf("被跳过的 a 必须还在队列里：%d", queue.Size())
	}

	queue.Release("red")
	chosen, ok = queue.Select(60000)
	if !ok || chosen.Name != "a" {
		t.Fatalf("名额释放后被挂起的 a 必须能复活：%+v %v", chosen, ok)
	}
	if queue.Size() != 0 {
		t.Fatalf("复活后队列应为空：%d", queue.Size())
	}
}

// 选取顺序：ReadyAt 升序，同刻按名字升序。
func TestSelectionOrder(t *testing.T) {
	queue := NewQueue(1000, 2)
	queue.Enqueue(Player{Name: "c", Team: "red", DeadAt: 20}) // ready 1020
	queue.Enqueue(Player{Name: "a", Team: "red", DeadAt: 10}) // ready 1010
	queue.Enqueue(Player{Name: "b", Team: "red", DeadAt: 10}) // ready 1010

	want := []string{"a", "b", "c"}
	for _, name := range want {
		chosen, ok := queue.Select(2000)
		if !ok || chosen.Name != name {
			t.Fatalf("顺序应为 a,b,c，本轮拿到 %+v %v", chosen, ok)
		}
	}
	if _, ok := queue.Select(2000); ok {
		t.Fatal("所有人复活后不该再选出人")
	}
}

// 选取代价：无人就绪时每次只看堆顶，5000 次查询的 scanned 必须在 5 万以内。
func TestSelectCostNoLinearScan(t *testing.T) {
	queue := NewQueue(1000, 100)
	for index := 0; index < 2000; index++ {
		queue.Enqueue(Player{Name: "p-" + itoa(index), Team: "red", DeadAt: int64(index)})
	}
	for query := 0; query < 5000; query++ {
		queue.Select(0)
	}
	if queue.Scanned() > 50000 {
		t.Fatalf("scanned 随在队人数线性增长：%d", queue.Scanned())
	}
	if queue.Size() != 2000 {
		t.Fatalf("无人就绪时 Size 不应变化：%d", queue.Size())
	}
}

// 满员被挂起的候选不能在后续 Select 里被反复扫描：只在名额释放时回归一次。
func TestBlockedCandidatesNotRescanned(t *testing.T) {
	queue := NewQueue(1000, 1)
	queue.Occupy("red", 1)
	for index := 0; index < 1000; index++ {
		queue.Enqueue(Player{Name: "p-" + itoa(index), Team: "red", DeadAt: 0})
	}

	if _, ok := queue.Select(5000); ok {
		t.Fatal("红队满员，不该选出任何人")
	}
	firstScan := queue.Scanned() // 第一次：把 1000 个就绪候选挂起
	if firstScan > 1001 {
		t.Fatalf("首轮挂起至多看过 1001 条：%d", firstScan)
	}
	for index := 0; index < 100; index++ {
		if _, ok := queue.Select(5000); ok {
			t.Fatal("红队仍满员，不该选出任何人")
		}
	}
	if got := queue.Scanned(); got != firstScan+100 {
		t.Fatalf("满员期间每次选取只该看堆顶（空堆），实际 scanned=%d", got)
	}

	queue.Release("red") // 名额释放，挂起候选回归主堆
	chosen, ok := queue.Select(5000)
	if !ok || chosen.Name != "p-0" {
		t.Fatalf("释放后同刻按名字升序应选 p-0：%+v %v", chosen, ok)
	}
}

// 同一批事件重复执行，结果完全相同。
func TestDeterministicReplay(t *testing.T) {
	build := func() *Queue {
		queue := NewQueue(1000, 2)
		queue.Occupy("red", 2)
		queue.Enqueue(Player{Name: "a", Team: "red", DeadAt: 0})
		queue.Enqueue(Player{Name: "b", Team: "blue", DeadAt: 5})
		queue.Enqueue(Player{Name: "a", Team: "red", DeadAt: 10})
		return queue
	}
	var runs [][]string
	for run := 0; run < 2; run++ {
		queue := build()
		var picked []string
		for _, now := range []int64{0, 1000, 1009, 1010, 6000} {
			if chosen, ok := queue.Select(now); ok {
				picked = append(picked, chosen.Name)
			}
			if now == 1000 {
				queue.Release("red")
			}
		}
		runs = append(runs, picked)
	}
	if len(runs[0]) != len(runs[1]) {
		t.Fatal("两次回放结果长度不同")
	}
	for index := range runs[0] {
		if runs[0][index] != runs[1][index] {
			t.Fatalf("同一批事件重复执行结果不同：%v vs %v", runs[0], runs[1])
		}
	}
}

// refPlayer 是参考实现里的一条记录。
type refPlayer struct {
	name string
	team string
	dead int64
}

// refSelect 是朴素参考实现：按最新记录合并 + 冷却含边界 + 队伍上限过滤，
// 每次全量扫描后选 ReadyAt 最小、同刻名字最小的人。
func refSelect(ref map[string]refPlayer, counts map[string]int, cooldown int64, teamCap int, now int64) (refPlayer, bool) {
	var candidates []refPlayer
	for _, player := range ref {
		if player.dead+cooldown > now {
			continue
		}
		if counts[player.team] >= teamCap {
			continue
		}
		candidates = append(candidates, player)
	}
	if len(candidates) == 0 {
		return refPlayer{}, false
	}
	sort.Slice(candidates, func(i, j int) bool {
		ri, rj := candidates[i].dead+cooldown, candidates[j].dead+cooldown
		if ri != rj {
			return ri < rj
		}
		return candidates[i].name < candidates[j].name
	})
	return candidates[0], true
}

// 200 组随机「死亡 + 队伍占用 + 选取时刻」序列，每次选取结果必须与参考实现一致。
func TestDifferential200(t *testing.T) {
	for seed := int64(1); seed <= 200; seed++ {
		rng := rand.New(rand.NewSource(seed))
		teams := []string{"red", "blue", "green"}
		teamCap := 1 + rng.Intn(3)
		cooldown := int64(500 + rng.Intn(2000))
		playerCount := 8 + rng.Intn(8)

		names := make([]string, playerCount)
		playerTeam := make(map[string]string, playerCount)
		lastDead := make(map[string]int64, playerCount)
		for i := 0; i < playerCount; i++ {
			names[i] = "p-" + itoa(i)
			playerTeam[names[i]] = teams[rng.Intn(len(teams))]
		}

		queue := NewQueue(cooldown, teamCap)
		ref := map[string]refPlayer{}
		counts := map[string]int{}
		var now int64

		for event := 0; event < 120; event++ {
			now += int64(rng.Intn(300))
			switch rng.Intn(10) {
			case 0, 1, 2, 3, 4: // 阵亡上报（同一玩家 DeadAt 严格递增）
				name := names[rng.Intn(playerCount)]
				lastDead[name] += 1 + int64(rng.Intn(400))
				player := Player{Name: name, Team: playerTeam[name], DeadAt: lastDead[name]}
				queue.Enqueue(player)
				ref[name] = refPlayer{name: name, team: player.Team, dead: player.DeadAt}
			case 5, 6: // 直接设置队伍在场人数
				team := teams[rng.Intn(len(teams))]
				count := rng.Intn(teamCap + 1)
				queue.Occupy(team, count)
				counts[team] = count
			case 7: // 释放一个名额
				team := teams[rng.Intn(len(teams))]
				queue.Release(team)
				if counts[team] > 0 {
					counts[team]--
				}
			default: // 选取
				got, gotOk := queue.Select(now)
				want, wantOk := refSelect(ref, counts, cooldown, teamCap, now)
				if gotOk != wantOk || (gotOk && got.Name != want.name) {
					t.Fatalf("seed=%d event=%d now=%d 选取不一致：got=(%s,%v) want=(%s,%v)",
						seed, event, now, got.Name, gotOk, want.name, wantOk)
				}
				if wantOk {
					delete(ref, want.name)
					if counts[want.team] > 0 {
						counts[want.team]--
					}
				}
			}
			if queue.Size() != len(ref) {
				t.Fatalf("seed=%d event=%d Size 不一致：got=%d want=%d",
					seed, event, queue.Size(), len(ref))
			}
		}
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits []byte
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
