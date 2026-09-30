package model

import "time"

// RawMember is the member profile dimension synced from ESB. Tier/JoinDate
// stay nil until a real ESB membership endpoint is wired up (see comment in
// the raw_tables migration) -- service-layer queries fall back to a
// spending-bracket rule for tier in the meantime.
type RawMember struct {
	MemberCode         string `gorm:"primaryKey"`
	MemberName         *string
	Tier               *string
	JoinDate           *time.Time
	HomeBranchCode     *string
	Status             *string
	ExternalMemberCode *string
	PhoneNumber        *string
	Email              *string
	Raw                []byte `gorm:"type:jsonb"`
	SyncedAt           time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
