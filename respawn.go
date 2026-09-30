// Package respawn 是复活队列：冷却台账、队伍在场人数上限与复活顺序选取。
package respawn

import "container/heap"

// DefaultCooldownMs 是默认复活冷却。
const DefaultCooldownMs = 10000

const (
	locHeap    = 0 // 条目在主堆里
	locBlocked = 1 // 条目已就绪但因队伍满员被挂起
)

// Player 是一条死亡记录。
type Player struct {
	Name   string
	Team   string
	DeadAt int64
}

// entry 是队列内部条目：同一玩家始终只有一个 entry，重复阵亡就地更新。
type entry struct {
	player  Player
	readyAt int64
	index   int
	loc     int
}

// Queue 是复活队列。teamCounts 记录每支队伍当前的在场人数。
//
// heap 里的条目按 (ReadyAt, Name) 排序；byName 负责按最新阵亡记录合并；
// blocked 挂起“已冷却但本队满员”的候选，名额释放时再整体放回主堆，
// 因此无人可复活时的选取只看堆顶，不随在队人数线性增长。
type Queue struct {
	cooldownMs int64
	teamCap    int
	heap       []*entry
	byName     map[string]*entry
	blocked    map[string][]*entry
	teamCounts map[string]int
	scanned    int
}

// NewQueue 建一个复活队列。teamCap 是每支队伍在场人数上限。
func NewQueue(cooldownMs int64, teamCap int) *Queue {
	return &Queue{
		cooldownMs: cooldownMs,
		teamCap:    teamCap,
		byName:     map[string]*entry{},
		blocked:    map[string][]*entry{},
		teamCounts: map[string]int{},
	}
}

// ReadyAt 是这个玩家最早能复活的时刻（含边界：now == ReadyAt 即可复活）。
func (q *Queue) ReadyAt(player Player) int64 { return player.DeadAt + q.cooldownMs }

// Enqueue 记一次死亡。同一 Name 的重复上报按最新记录合并，冷却与顺序都按最新那条算。
func (q *Queue) Enqueue(player Player) {
	readyAt := q.ReadyAt(player)
	if existing := q.byName[player.Name]; existing != nil {
		existing.player = player
		existing.readyAt = readyAt
		if existing.loc == locHeap {
			heap.Fix(q, existing.index)
		}
		return
	}
	node := &entry{player: player, readyAt: readyAt, index: len(q.heap), loc: locHeap}
	q.byName[player.Name] = node
	heap.Push(q, node)
}

// Occupy 直接设置一支队伍的在场人数（用于模拟队伍里已经站着的队友）。
func (q *Queue) Occupy(team string, count int) {
	q.teamCounts[team] = count
	if count < q.teamCap {
		q.flushBlocked(team)
	}
}

// Release 在复活成功后释放一个队伍名额；被满员挂起的本队候选重新参与选取。
func (q *Queue) Release(team string) {
	if q.teamCounts[team] > 0 {
		q.teamCounts[team]--
	}
	if q.teamCounts[team] < q.teamCap {
		q.flushBlocked(team)
	}
}

// flushBlocked 把某队被挂起的候选全部放回主堆。
func (q *Queue) flushBlocked(team string) {
	items := q.blocked[team]
	for _, node := range items {
		node.loc = locHeap
		heap.Push(q, node)
	}
	if len(items) > 0 {
		delete(q.blocked, team)
	}
}

// Select 挑出本轮该复活的玩家，并把它从队列里移除。
//
// 只从堆顶取就绪候选（now >= ReadyAt，含边界）；本队满员则挂起该候选继续看下一个，
// 选中后调用 Release 归还名额。堆顶尚未就绪即说明没有可复活的人，返回 false。
func (q *Queue) Select(nowMs int64) (Player, bool) {
	for {
		q.scanned++
		if len(q.heap) == 0 {
			return Player{}, false
		}
		top := q.heap[0]
		if top.readyAt > nowMs {
			return Player{}, false
		}
		heap.Pop(q)
		if q.teamCounts[top.player.Team] >= q.teamCap {
			top.loc = locBlocked
			q.blocked[top.player.Team] = append(q.blocked[top.player.Team], top)
			continue
		}
		chosen := top.player
		delete(q.byName, chosen.Name)
		q.Release(chosen.Team)
		return chosen, true
	}
}

// Size 是队列里还在等复活的不同玩家数（主堆与满员挂起合计）。
func (q *Queue) Size() int { return len(q.byName) }

// Scanned 是 Select 累计看过的记录数（规模观测）。
func (q *Queue) Scanned() int { return q.scanned }

// Len/Less/Swap/Push/Pop 实现 container/heap.Interface，顺序为 ReadyAt 升序、同刻名字升序。
func (q *Queue) Len() int { return len(q.heap) }

func (q *Queue) Less(i, j int) bool {
	left, right := q.heap[i], q.heap[j]
	if left.readyAt != right.readyAt {
		return left.readyAt < right.readyAt
	}
	return left.player.Name < right.player.Name
}

func (q *Queue) Swap(i, j int) {
	q.heap[i], q.heap[j] = q.heap[j], q.heap[i]
	q.heap[i].index = i
	q.heap[j].index = j
}

func (q *Queue) Push(value any) {
	node := value.(*entry)
	node.index = len(q.heap)
	q.heap = append(q.heap, node)
}

func (q *Queue) Pop() any {
	old := q.heap
	last := len(old) - 1
	node := old[last]
	old[last] = nil
	q.heap = old[:last]
	return node
}
