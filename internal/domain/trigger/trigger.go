package trigger

import apptrig "github.com/R3E-Network/service_layer/internal/app/domain/trigger"

type (
	Trigger = apptrig.Trigger
	Type    = apptrig.Type
)

const (
	TypeCron    = apptrig.TypeCron
	TypeEvent   = apptrig.TypeEvent
	TypeWebhook = apptrig.TypeWebhook
)
