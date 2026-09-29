package respawn

import "testing"

func TestPicksNearestClearPoint(t *testing.T) {
	manager := &Manager{CooldownMs: 10000}
	manager.Die(0, 0, 1000)
	point, err := manager.Respawn(20000, []SpawnPoint{
		{Name: "far", X: 7, Y: 0},
		{Name: "near", X: 1, Y: 0},
	})
	if err != nil {
		t.Fatalf("冷却走完以后应该能复活：%v", err)
	}
	if point.Name != "near" {
		t.Fatalf("应该挑最近的复活点：%s", point.Name)
	}
}

func TestRespawnClearsDeadFlag(t *testing.T) {
	manager := &Manager{CooldownMs: 10000}
	manager.Die(4, 4, 0)
	if !manager.Dead() {
		t.Fatal("Die 之后应该处于死亡状态")
	}
	if _, err := manager.Respawn(60000, []SpawnPoint{{Name: "a"}}); err != nil {
		t.Fatalf("复活失败：%v", err)
	}
	if manager.Dead() {
		t.Fatal("复活之后不应该还处于死亡状态")
	}
}

func TestSafeRadiusConstant(t *testing.T) {
	if SafeRadius != 12 {
		t.Fatal("安全半径被改了")
	}
}
