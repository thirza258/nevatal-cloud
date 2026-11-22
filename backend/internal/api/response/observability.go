package response

import (
	"github.com/nevatal/cloud/internal/usecase"
)

type AdminObservabilityOverview = usecase.AdminObservabilityOverview
type AdminKPI = usecase.AdminKPI
type UserObservabilityOverview = usecase.UserObservabilityOverview
type VMHealthScore = usecase.VMHealthScore
type MetricTrend = usecase.MetricTrend
type ZFSTrend = usecase.ZFSTrend
type InfrastructureHealth = usecase.InfrastructureHealth
type APIEndpointStats = usecase.APIEndpointStats
type APIObservabilitySummary = usecase.APIObservabilitySummary
type JobTimelineStep = usecase.JobTimelineStep
type JobFailureDetail = usecase.JobFailureDetail
type JobObservabilitySummary = usecase.JobObservabilitySummary
type AIObservabilitySummary = usecase.AIObservabilitySummary
type TenantObservabilityDetail = usecase.TenantObservabilityDetail
type CapacityForecast = usecase.CapacityForecast
type AIIncidentInvestigationResult = usecase.AIIncidentInvestigationResult
type VMDetailObservability = usecase.VMDetailObservability
