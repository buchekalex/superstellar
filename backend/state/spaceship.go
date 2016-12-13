package state

import (
	"fmt"
	"math"
	"superstellar/backend/constants"
	"superstellar/backend/pb"
	"superstellar/backend/types"
	"superstellar/backend/utils"
	"time"
)

// Direction is a type describing user input on spaceship rotation.
type Direction int

// Constants describing user input on spaceship rotation.
const (
	NONE Direction = iota
	RIGHT
	LEFT
)

// Spaceship struct describes a spaceship.
type Spaceship struct {
	ID                   uint32
	Position             *types.Point
	Velocity             *types.Vector
	Facing               float64
	AngularVelocity      float64
	AngularVelocityDelta float64
	InputThrust          bool
	InputBoost           bool
	InputDirection       Direction
	TargetAngle          *float64
	Fire                 bool
	LastShotTime         time.Time
	Dirty                bool
	DirtyFramesTimeout   uint32
	LastSentOn           uint32
	HP                   uint32
	MaxHP                uint32
	Energy               uint32
	MaxEnergy            uint32
	AutoRepairDelay      uint32
}

func NewSpaceship(clientId uint32, initialPosition *types.Point) *Spaceship {
	return &Spaceship{
		ID:                   clientId,
		Position:             initialPosition,
		Velocity:             types.ZeroVector(),
		Facing:               0.0,
		AngularVelocity:      0,
		AngularVelocityDelta: 0,
		InputThrust:          false,
		InputDirection:       NONE,
		Fire:                 false,
		LastShotTime:         time.Now(),
		Dirty:                true,
		DirtyFramesTimeout:   0,
		LastSentOn:           0,
		HP:                   constants.SpaceshipInitialHP,
		MaxHP:                constants.SpaceshipInitialHP,
		Energy:               constants.SpaceshipInitialEnergy,
		MaxEnergy:            constants.SpaceshipInitialEnergy,
		AutoRepairDelay:      constants.AutoRepairDelay,
	}
}

// String function returns string representation.
func (s *Spaceship) String() string {
	return fmt.Sprintf("(%v, %v, %v)", s.Position, s.Velocity, s.Facing)
}

func (s *Spaceship) UpdateUserInput(userInput pb.UserInput) {
	switch userInput {
	case pb.UserInput_CENTER:
		s.InputDirection = NONE
		s.TargetAngle = nil
		s.MarkDirty()
	case pb.UserInput_LEFT:
		s.InputDirection = LEFT
		s.TargetAngle = nil
		s.MarkDirty()
	case pb.UserInput_RIGHT:
		s.InputDirection = RIGHT
		s.TargetAngle = nil
		s.MarkDirty()
	case pb.UserInput_THRUST_ON:
		s.InputThrust = true
		s.MarkDirty()
	case pb.UserInput_THRUST_OFF:
		s.InputThrust = false
		s.MarkDirty()
	case pb.UserInput_FIRE_START:
		s.Fire = true
		s.MarkDirty()
	case pb.UserInput_FIRE_STOP:
		s.Fire = false
	case pb.UserInput_BOOST_ON:
		s.InputThrust = true
		s.InputBoost = true
		s.MarkDirty()
	case pb.UserInput_BOOST_OFF:
		s.InputThrust = false
		s.InputBoost = false
		s.MarkDirty()
	}
}

func (s *Spaceship) MarkDirty() {
	s.Dirty = true
	s.DirtyFramesTimeout = constants.DirtyFramesTimeout
}

func (s *Spaceship) NotifyAboutNewFrame() {
	s.handleDirtyTimeout()
	s.handleAutoEnergyRecharge()
	s.handleAutoRepair()
}

func (s *Spaceship) UpdateTargetAngle(angle float64) {
	s.Dirty = true
	s.TargetAngle = &angle
	s.InputDirection = NONE
}

// ToProto returns protobuf representation
func (s *Spaceship) ToProto() *pb.Spaceship {
	return &pb.Spaceship{
		Id:              s.ID,
		Position:        s.Position.ToProto(),
		Velocity:        s.Velocity.ToProto(),
		Facing:          s.Facing,
		AngularVelocity: s.AngularVelocity,
		InputDirection:  pb.Direction(s.InputDirection),
		InputThrust:     s.InputThrust,
		InputBoost:      s.InputBoost,
		MaxHp:           s.MaxHP,
		Hp:              s.HP,
		MaxEnergy:       s.MaxEnergy,
		Energy:          s.Energy,
		AutoRepairDelay: s.AutoRepairDelay,
	}
}

// DetectCollision returns true if receiver spaceship collides with other spaceship.
func (s *Spaceship) DetectCollision(other *Spaceship) bool {
	v := types.Point{X: s.Position.X - other.Position.X, Y: s.Position.Y - other.Position.Y}
	dist := v.Length()

	return dist < 2*constants.SpaceshipSize
}

// Collide transforms colliding ships' parameters.
func (s *Spaceship) Collide(other *Spaceship) {
	v := types.Point{
		X: s.Position.X - other.Position.X,
		Y: s.Position.Y - other.Position.Y,
	}

	transformAngle := -math.Atan2(float64(v.Y), float64(v.X))
	newV1 := s.Velocity.Rotate(transformAngle)
	newV2 := other.Velocity.Rotate(transformAngle)

	switchedV1 := types.Vector{X: newV2.X, Y: newV1.Y}
	switchedV2 := types.Vector{X: newV1.X, Y: newV2.Y}

	s.Velocity = switchedV1.Rotate(-transformAngle)
	other.Velocity = switchedV2.Rotate(-transformAngle)

	s.MarkDirty()
	other.MarkDirty()
}

func (s *Spaceship) ShootIfPossible() (canShoot bool) {
	if s.Energy >= constants.BasicWeaponEnergyCost {
		canShoot = true
		s.Energy -= constants.BasicWeaponEnergyCost
		s.Dirty = true
	} else {
		canShoot = false
	}
	return canShoot
}

func (s *Spaceship) CollideWithProjectile(projectile *Projectile) {
	if s.HP < constants.ProjectileDamage {
		s.HP = 0
	} else {
		s.HP -= constants.ProjectileDamage
	}
	s.AutoRepairDelay = constants.AutoRepairDelay

	s.MarkDirty()
}

func (s *Spaceship) AddReward(reward uint32) {
	s.HP += reward
	s.MaxHP += reward

	s.MarkDirty()
}

func (s *Spaceship) AddEnergyReward(reward uint32) {
	s.Energy += reward
	s.MaxEnergy += reward

	s.MarkDirty()
}

func (s *Spaceship) handleDirtyTimeout() {
	if s.DirtyFramesTimeout == 0 {
		s.MarkDirty()
	} else {
		s.DirtyFramesTimeout--
	}
}

func (s *Spaceship) handleAutoRepair() {
	if s.AutoRepairDelay == 0 {
		s.HP = utils.Min(s.HP+constants.AutoRepairAmount, s.MaxHP)
	} else {
		s.AutoRepairDelay--
	}
}

func (s *Spaceship) handleAutoEnergyRecharge() {
	s.Energy = utils.Min(s.Energy+constants.AutoEnergyRechargeAmount, s.MaxEnergy)
}

func (s *Spaceship) LeftTurn() {
	s.AngularVelocityDelta = s.angularVelocityDelta()
	s.LimitAngularVelocityDelta()
}

func (s *Spaceship) RightTurn() {
	s.AngularVelocityDelta = -s.angularVelocityDelta()
	s.LimitAngularVelocityDelta()
}

func (s *Spaceship) TurnToTarget() {
	targetAngle := *s.TargetAngle
	offset := targetAngle - s.Facing

	if math.Abs(offset) > math.Pi {
		offset -= math.Copysign(2*math.Pi, offset)
	}

	targetAngularVelocity := -offset * constants.SpaceshipTurnToAngleP
	s.AngularVelocityDelta = targetAngularVelocity - s.AngularVelocity

	s.LimitAngularVelocityDelta()
}

func (s *Spaceship) LimitAngularVelocityDelta() {
	potentialAngularVelocity := s.AngularVelocity + s.AngularVelocityDelta
	diff := math.Abs(potentialAngularVelocity) - constants.SpaceshipMaxAngularVelocity

	if diff > 0 {
		s.AngularVelocityDelta -= math.Copysign(diff, s.AngularVelocity)
	}
}

func (s *Spaceship) ApplyAngularFriction() {
	s.AngularVelocity *= (1 - constants.SpaceshipAngularFriction)
}

func (s *Spaceship) angularVelocityDelta() float64 {
	nonlinearPart := constants.SpaceshipNonlinearAngularAcceleration * math.Abs(s.AngularVelocity)
	linearPart := constants.SpaceshipLinearAngularAcceleration
	return nonlinearPart + linearPart
}
