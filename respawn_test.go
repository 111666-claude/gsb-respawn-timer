package respawn

import "testing"

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
