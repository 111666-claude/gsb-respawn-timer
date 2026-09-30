// Package respawn 是复活队列：冷却台账、队伍在场人数上限与复活顺序选取。
//
// 数据结构（只用标准库 container/heap）：
//   - entries：Name -> 阵亡记录，同一玩家只保留按最新 DeadAt 合并后的一条；
//   - 每支队伍一个按 (ReadyAt, Name) 排序的最小堆，维护堆内下标可原地更新；
//   - 一个全局最小堆，里面只有“在场人数未达 TeamCap”的队伍的队头。
//
// 因此 Select 每次只需要 pop 一个队头，代价 O(log n)，既不会全表扫描，
// 也不会因为某些队伍满员而线性跳过候选；满员队伍的队头根本不在全局堆里。
package respawn

import "container/heap"

// DefaultCooldownMs 是默认复活冷却。
const DefaultCooldownMs = 10000

// Player 是一条死亡记录。
type Player struct {
	Name   string
	Team   string
	DeadAt int64
}

// entry 是队列内部的阵亡记录节点，同时挂在所属队伍的堆里。
type entry struct {
	player Player
	readyAt int64
	// teamIndex 是该节点在所属队伍堆里的下标，-1 表示不在堆中。
	teamIndex int
}

// teamState 记录一支队伍的在场人数和该队的等待队列。
type teamState struct {
	count int
	heap  teamHeap
	// globalIndex 是该队队头在全局堆里的下标，-1 表示不在全局堆中。
	globalIndex int
}

// globalNode 是全局堆里的一个节点，指向一支队伍的队头。
type globalNode struct {
	team    *teamState
	readyAt int64
	name    string
	index   int
}

// Queue 是复活队列。teamCounts 记录每支队伍当前的在场人数。
type Queue struct {
	cooldownMs int64
	teamCap    int
	entries    map[string]*entry
	teams      map[string]*teamState
	global     globalHeap
	scanned    int
}

// NewQueue 建一个复活队列。teamCap 是每支队伍在场人数上限。
func NewQueue(cooldownMs int64, teamCap int) *Queue {
	return &Queue{
		cooldownMs: cooldownMs,
		teamCap:    teamCap,
		entries:    make(map[string]*entry),
		teams:      make(map[string]*teamState),
		global:     make(globalHeap, 0),
	}
}

// ReadyAt 是这个玩家最早能复活的时刻。
func (q *Queue) ReadyAt(player Player) int64 { return player.DeadAt + q.cooldownMs }

// Enqueue 记一次死亡。同一玩家重复阵亡按最新 DeadAt 合并到同一条记录，
// 冷却与选取顺序都按最新那条算（含跨队伍的更新）。
func (q *Queue) Enqueue(player Player) {
	if existing, ok := q.entries[player.Name]; ok {
		if existing.player.Team != player.Team {
			oldTeam := q.teams[existing.player.Team]
			heap.Remove(&oldTeam.heap, existing.teamIndex)
			q.syncTeam(oldTeam)
			existing.teamIndex = -1
		}
		existing.player = player
		existing.readyAt = q.ReadyAt(player)
		team := q.teamOf(player.Team)
		if existing.teamIndex < 0 {
			existing.teamIndex = len(team.heap)
			team.heap = append(team.heap, existing)
		}
		heap.Fix(&team.heap, existing.teamIndex)
		q.syncTeam(team)
		return
	}
	team := q.teamOf(player.Team)
	node := &entry{player: player, readyAt: q.ReadyAt(player), teamIndex: len(team.heap)}
	team.heap = append(team.heap, node)
	heap.Fix(&team.heap, node.teamIndex)
	q.entries[player.Name] = node
	q.syncTeam(team)
}

// Occupy 直接设置一支队伍的在场人数（用于模拟队伍里已经站着的队友）。
func (q *Queue) Occupy(team string, count int) {
	state := q.teamOf(team)
	state.count = count
	q.syncTeam(state)
}

// Release 在复活成功后释放一个队伍名额。
func (q *Queue) Release(team string) {
	state, ok := q.teams[team]
	if !ok {
		return
	}
	if state.count > 0 {
		state.count--
	}
	q.syncTeam(state)
}

// Select 挑出本轮该复活的玩家，并把它从队列里移除。
// 冷却含边界：now >= ReadyAt 即可复活；满员队伍的候选直接跳过（不进全局堆）；
// 顺序按 ReadyAt 升序、同刻按名字升序；没有可复活的人返回 false。
func (q *Queue) Select(nowMs int64) (Player, bool) {
	if q.global.Len() == 0 {
		return Player{}, false
	}
	node := q.global[0]
	if node.readyAt > nowMs {
		return Player{}, false
	}
	q.scanned++
	heap.Pop(&q.global)
	node.team.globalIndex = -1

	state := node.team
	chosen := heap.Pop(&state.heap).(*entry)
	chosen.teamIndex = -1
	delete(q.entries, chosen.player.Name)
	q.syncTeam(state)

	// 复活成功后把名额还回去（README 口径）。
	if state.count > 0 {
		state.count--
	}
	q.syncTeam(state)
	return chosen.player, true
}

// Size 是队列里还在等复活的、不同玩家的数量。
func (q *Queue) Size() int { return len(q.entries) }

// Scanned 是 Select 累计看过的记录数（规模观测）。
// 每次成功选取只检查一个全局队头，摊还 O(1)（堆维护为 O(log n)），
// 不随在队人数线性增长。
func (q *Queue) Scanned() int { return q.scanned }

// teamOf 取（或新建）一支队伍的状态。
func (q *Queue) teamOf(team string) *teamState {
	state, ok := q.teams[team]
	if !ok {
		state = &teamState{globalIndex: -1}
		state.heap = make(teamHeap, 0)
		q.teams[team] = state
	}
	return state
}

// syncTeam 保证一支队伍“在场人数未满且队列非空”时，
// 它的队头（ReadyAt 最小、同刻名字最小）出现在全局堆里，否则不在。
func (q *Queue) syncTeam(state *teamState) {
	eligible := state.count < q.teamCap && state.heap.Len() > 0
	switch {
	case eligible && state.globalIndex < 0:
		head := state.heap[0]
		heap.Push(&q.global, &globalNode{
			team:      state,
			readyAt:   head.readyAt,
			name:      head.player.Name,
			index:     -1,
		})
		state.globalIndex = q.global.Len() - 1
	case !eligible && state.globalIndex >= 0:
		heap.Remove(&q.global, state.globalIndex)
		state.globalIndex = -1
	case eligible && state.globalIndex >= 0:
		head := state.heap[0]
		node := q.global[state.globalIndex]
		if node.readyAt != head.readyAt || node.name != head.player.Name {
			node.readyAt = head.readyAt
			node.name = head.player.Name
			heap.Fix(&q.global, state.globalIndex)
		}
	}
}

// teamHeap 是一支队伍内部按 (ReadyAt, Name) 排序的最小堆。
type teamHeap []*entry

func (h teamHeap) Len() int { return len(h) }

func (h teamHeap) Less(i, j int) bool {
	if h[i].readyAt != h[j].readyAt {
		return h[i].readyAt < h[j].readyAt
	}
	return h[i].player.Name < h[j].player.Name
}

func (h teamHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].teamIndex = i
	h[j].teamIndex = j
}

// Push 向队伍堆里追加一个节点。
func (h *teamHeap) Push(value any) {
	node := value.(*entry)
	node.teamIndex = len(*h)
	*h = append(*h, node)
}

// Pop 取出队伍堆的堆顶节点。
func (h *teamHeap) Pop() any {
	old := *h
	n := len(old)
	node := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	node.teamIndex = -1
	return node
}

// globalHeap 是所有“未满员”队伍队头的全局最小堆。
type globalHeap []*globalNode

func (h globalHeap) Len() int { return len(h) }

func (h globalHeap) Less(i, j int) bool {
	if h[i].readyAt != h[j].readyAt {
		return h[i].readyAt < h[j].readyAt
	}
	return h[i].name < h[j].name
}

func (h globalHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

// Push 向全局堆追加一个队伍队头节点。
func (h *globalHeap) Push(value any) {
	node := value.(*globalNode)
	node.index = len(*h)
	*h = append(*h, node)
}

// Pop 取出全局堆的堆顶节点。
func (h *globalHeap) Pop() any {
	old := *h
	n := len(old)
	node := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	node.index = -1
	return node
}
