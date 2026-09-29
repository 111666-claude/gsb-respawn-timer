// Package respawn 是死亡与复活：冷却校验、安全点挑选、状态清理。
// 缺陷：复活不看冷却，不跳过被敌人占住的点，也没有安全点时报不出错。
package respawn

import (
	"errors"
	"math"
)

// SafeRadius 是安全点的保护半径，米。
const SafeRadius = 12.0

var (
	// ErrNotDead 表示当前没有死亡记录。
	ErrNotDead = errors.New("respawn: 没有死亡记录")
	// ErrCooldown 表示复活冷却还没走完。
	ErrCooldown = errors.New("respawn: 复活冷却中")
	// ErrNoSafe 表示一个安全点都没有。
	ErrNoSafe = errors.New("respawn: 没有可用安全点")
)

// SpawnPoint 是一个复活点。Enemies 是半径 SafeRadius 内的敌对单位数。
type SpawnPoint struct {
	Name    string
	X, Y    float64
	Enemies int
}

// Manager 是复活管理器。
type Manager struct {
	CooldownMs int64

	deadAt       int64
	deadX, deadY float64
	dead         bool
}

// Die 记录一次死亡。缺陷：位置记下来了，但复活时没人用。
func (m *Manager) Die(x, y float64, atMs int64) {
	m.deadAt = atMs
	m.deadX = x
	m.deadY = y
	m.dead = true
}

// Dead 是当前是否处于死亡状态。
func (m *Manager) Dead() bool { return m.dead }

// SafePoints 返回没有敌人的复活点，顺序与输入一致。
// 缺陷：没有过滤 Enemies，被敌人占住的点也会出现在结果里。
func (m *Manager) SafePoints(points []SpawnPoint) []SpawnPoint {
	safe := make([]SpawnPoint, 0, len(points))
	for _, point := range points {
		safe = append(safe, point)
	}
	return safe
}

// Respawn 挑一个安全点复活。
// 缺陷：不校验冷却与死亡状态，不跳过有敌人的点，没有点时返回零值点而不报错。
func (m *Manager) Respawn(atMs int64, points []SpawnPoint) (SpawnPoint, error) {
	best := SpawnPoint{}
	bestDistance := math.MaxFloat64
	for _, point := range points {
		distance := math.Hypot(point.X-m.deadX, point.Y-m.deadY)
		if distance < bestDistance {
			best, bestDistance = point, distance
		}
	}
	m.dead = false
	return best, nil
}
