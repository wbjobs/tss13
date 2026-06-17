package server

import "math/rand"

type CombatResult struct {
	Hit          bool
	Damage       int
	DefenderHP   int
	DefenderDead bool
	Crit         bool
	Roll         int
}

func DiceRoll(n, sides int) int {
	total := 0
	for i := 0; i < n; i++ {
		total += rand.Intn(sides) + 1
	}
	return total
}

func AttackRoll(attackerAttack, defenderDefense int) (bool, int) {
	roll := DiceRoll(1, 20)
	if roll == 20 {
		return true, roll
	}
	if roll == 1 {
		return false, roll
	}
	total := roll + attackerAttack
	return total >= 10+defenderDefense, roll
}

func DamageRoll(attackPower int) int {
	return DiceRoll(1, 6) + attackPower/3
}

func PlayerAttackMonster(player *Player, monster *Monster) CombatResult {
	hit, roll := AttackRoll(player.Attack, monster.Defense)
	result := CombatResult{
		Hit:  hit,
		Roll: roll,
	}
	if !hit {
		result.Damage = 0
		result.DefenderHP = monster.HP
		result.DefenderDead = !monster.Alive
		return result
	}
	damage := DamageRoll(player.Attack)
	crit := roll == 20
	if crit {
		damage *= 2
		result.Crit = true
	}
	monster.HP -= damage
	result.Damage = damage
	result.DefenderHP = monster.HP
	if monster.HP <= 0 {
		monster.Alive = false
		result.DefenderDead = true
		player.XP += monster.XPValue
		if player.XP >= player.XPToNext {
			LevelUp(player)
		}
	} else {
		result.DefenderDead = false
	}
	return result
}

func MonsterAttackPlayer(monster *Monster, player *Player) CombatResult {
	hit, roll := AttackRoll(monster.Attack, player.Defense)
	result := CombatResult{
		Hit:  hit,
		Roll: roll,
	}
	if !hit {
		result.Damage = 0
		result.DefenderHP = player.HP
		result.DefenderDead = player.HP <= 0
		return result
	}
	damage := DamageRoll(monster.Attack)
	crit := roll == 20
	if crit {
		damage *= 2
		result.Crit = true
	}
	player.HP -= damage
	result.Damage = damage
	result.DefenderHP = player.HP
	if player.HP <= 0 {
		player.Alive = false
		result.DefenderDead = true
	} else {
		result.DefenderDead = false
	}
	return result
}

func LevelUp(player *Player) {
	player.Level++
	player.XPToNext = player.XPToNext * 3 / 2
	player.MaxHP += 5
	player.HP = player.MaxHP
	player.Attack++
	player.Defense++
}

func ProcessMonsterAttacks(monsters map[int]*Monster, players map[int]*Player) []CombatResult {
	var results []CombatResult
	for _, monster := range monsters {
		if !monster.Alive || monster.AIState != "attack" {
			continue
		}
		player, ok := players[monster.TargetID]
		if !ok || !player.Alive {
			continue
		}
		dx := monster.X - player.X
		dy := monster.Y - player.Y
		if dx < 0 {
			dx = -dx
		}
		if dy < 0 {
			dy = -dy
		}
		if dx <= 1 && dy <= 1 {
			result := MonsterAttackPlayer(monster, player)
			results = append(results, result)
		}
	}
	return results
}
