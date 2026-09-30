// Package respawn 是复活队列：冷却台账、队伍在场人数上限与复活顺序选取。
// 缺陷：冷却边界差一、同一玩家重复入队不合并、不看队伍人数上限、选取是全表线性扫描。
package respawn

// DefaultCooldownMs 是默认复活冷却。
const DefaultCooldownMs = 10000

// Player 是一条死亡记录。
type Player struct {
	Name   string
	Team   string
	DeadAt int64
}

// Queue 是复活队列。teamCounts 记录每支队伍当前的在场人数。
type Queue struct {
	cooldownMs int64
	teamCap    int
	entries    []Player
	teamCounts map[string]int
	scanned    int
}

// NewQueue 建一个复活队列。teamCap 是每支队伍在场人数上限。
func NewQueue(cooldownMs int64, teamCap int) *Queue {
	return &Queue{cooldownMs: cooldownMs, teamCap: teamCap, teamCounts: map[string]int{}}
}

// ReadyAt 是这个玩家最早能复活的时刻。
func (q *Queue) ReadyAt(player Player) int64 { return player.DeadAt + q.cooldownMs }

// Enqueue 记一次死亡。
// 缺陷：同一玩家重复死亡会留下多条记录，冷却与顺序都会按旧记录算。
func (q *Queue) Enqueue(player Player) {
	q.entries = append(q.entries, player)
}

// Occupy 直接设置一支队伍的在场人数（用于模拟队伍里已经站着的队友）。
func (q *Queue) Occupy(team string, count int) { q.teamCounts[team] = count }

// Release 在复活成功后释放一个队伍名额。
func (q *Queue) Release(team string) {
	if q.teamCounts[team] > 0 {
		q.teamCounts[team]--
	}
}

// Select 挑出本轮该复活的玩家，并把它从队列里移除。
// 缺陷：用严格大于判冷却（正好到点的人选不出来）、完全不看队伍在场人数上限、
// 每次都从头线性扫一遍队列。
func (q *Queue) Select(nowMs int64) (Player, bool) {
	q.scanned += len(q.entries)
	best := -1
	var bestReady int64
	for index, player := range q.entries {
		readyAt := q.ReadyAt(player)
		if nowMs <= readyAt {
			continue
		}
		if best < 0 || readyAt < bestReady || (readyAt == bestReady && player.Name < q.entries[best].Name) {
			best, bestReady = index, readyAt
		}
	}
	if best < 0 {
		return Player{}, false
	}
	chosen := q.entries[best]
	q.entries = append(q.entries[:best], q.entries[best+1:]...)
	return chosen, true
}

// Size 是队列里还在等复活的记录数。
func (q *Queue) Size() int { return len(q.entries) }

// Scanned 是 Select 累计看过的记录数（规模观测）。
func (q *Queue) Scanned() int { return q.scanned }
