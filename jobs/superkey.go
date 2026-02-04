package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/RedHatInsights/sources-api-go/dao"
	"github.com/RedHatInsights/sources-api-go/internal/superkey"
	"github.com/RedHatInsights/sources-api-go/kafka"
	l "github.com/RedHatInsights/sources-api-go/logger"
	m "github.com/RedHatInsights/sources-api-go/model"
	"github.com/RedHatInsights/sources-api-go/service"
	"github.com/RedHatInsights/sources-api-go/util"
)

type SuperkeyDestroyJob struct {
	Headers  []kafka.Header `json:"headers"`
	Identity string         `json:"identity"`
	Tenant   int64          `json:"tenant"`
	Model    string         `json:"model"`
	Id       int64          `json:"id"`
}

func (sk SuperkeyDestroyJob) Delay() time.Duration {
	// run this job immediately, no delay.
	return 0
}

func (sk SuperkeyDestroyJob) Arguments() map[string]interface{} {
	return map[string]interface{}{
		"model": sk.Model,
		"id":    sk.Id,
	}
}

func (sk SuperkeyDestroyJob) Name() string {
	return "SuperkeyDestroyJob"
}

func (sk SuperkeyDestroyJob) Run() error {
	l.Log.Infof("Running [%v] with [%v]", sk.Name(), sk.Arguments())

	switch sk.Model {
	case "source":
		return sk.sendForSource(sk.Id)
	case "application":
		return sk.sendForApplication(sk.Id)
	default:
		return fmt.Errorf("unsupported model for superkey: %v", sk.Model)
	}
}

// Lists all applications for a source, sends the destroy request for each, then
// enqueues deletion of itself.

// the sub-jobs enqueue deletion of their respective resources.
func (sk SuperkeyDestroyJob) sendForSource(id int64) error {
	l.Log.Infof("Sending SuperKey Delete request for source %v", sk.Id)

	a := dao.GetApplicationDao(&dao.RequestParams{TenantID: &sk.Tenant})

	apps, _, err := a.SubCollectionList(m.Source{ID: id}, 100, 0, make([]util.Filter, 0))
	if err != nil {
		return fmt.Errorf("failed to list applications for source: %v", err)
	}

	errors := make([]error, 0)

	for i := range apps {
		err := sk.sendForApplication(apps[i].ID)
		if err != nil {
			l.Log.Warnf("Error sending destroy request for application %v: %v", apps[i].ID, err)
			errors = append(errors, err)
		}
	}

	if len(errors) != 0 {
		return fmt.Errorf("ran into errors sending delete requests for application: %v", errors)
	}

	// destroy the source after waiting 15 seconds
	Enqueue(&AsyncDestroyJob{
		Headers:     sk.Headers,
		Tenant:      sk.Tenant,
		WaitSeconds: 15,
		Model:       "source",
		Id:          id,
	})

	return nil
}

func (sk SuperkeyDestroyJob) sendForApplication(id int64) error {
	l.Log.Infof("Destroying SuperKey resources for application %v", id)

	// Load the application
	appDao := dao.GetApplicationDao(&dao.RequestParams{TenantID: &sk.Tenant})
	app, err := appDao.GetById(&id)
	if err != nil {
		l.Log.Warnf("Failed to load application %d: %v", id, err)
		// Continue with deletion even if we can't load the app
	} else {
		// Build destroy request
		req, err := buildDestroyRequest(app)
		if err != nil {
			l.Log.Warnf("Failed to build destroy request: %v", err)
			// Continue with deletion even if we can't build the request
		} else if req != nil {
			// Destroy AWS resources via orchestrator
			orch := superkey.NewOrchestrator(sk.Tenant)
			ctx := context.Background()

			errors, err := orch.DestroyResources(ctx, req)
			if err != nil {
				l.Log.Errorf("Failed to destroy resources: %v", err)
			}
			if len(errors) > 0 {
				l.Log.Warnf("Partial teardown failures: %v", errors)
			}
		}
	}

	// Enqueue async deletion of the application (after 15 seconds)
	Enqueue(&AsyncDestroyJob{
		Headers:     sk.Headers,
		Tenant:      sk.Tenant,
		WaitSeconds: 15,
		Model:       "application",
		Id:          id,
	})

	return nil
}

func (sk SuperkeyDestroyJob) ToJSON() []byte {
	bytes, err := json.Marshal(&sk)
	if err != nil {
		panic(err)
	}

	return bytes
}

// SuperkeyCreateJob handles the creation of superkey resources for an application
type SuperkeyCreateJob struct {
	ApplicationID int64          `json:"application_id"`
	Tenant        int64          `json:"tenant"`
	Headers       []kafka.Header `json:"headers"`
}

func (sk SuperkeyCreateJob) Delay() time.Duration {
	// Run immediately
	return 0
}

func (sk SuperkeyCreateJob) Arguments() map[string]interface{} {
	return map[string]interface{}{
		"application_id": sk.ApplicationID,
		"tenant":         sk.Tenant,
	}
}

func (sk SuperkeyCreateJob) Name() string {
	return "SuperkeyCreateJob"
}

func (sk SuperkeyCreateJob) Run() error {
	l.Log.Infof("Running [%v] with [%v]", sk.Name(), sk.Arguments())

	// Get application DAO
	appDao := dao.GetApplicationDao(&dao.RequestParams{TenantID: &sk.Tenant})

	// Load application with associations
	app, err := appDao.GetByIdWithPreload(&sk.ApplicationID, "Tenant", "Source", "ApplicationType")
	if err != nil {
		l.Log.Errorf("Failed to load application %d: %v", sk.ApplicationID, err)
		return fmt.Errorf("failed to load application: %w", err)
	}

	// Build the create request
	req, err := buildCreateRequest(app)
	if err != nil {
		l.Log.Errorf("Failed to build create request: %v", err)
		// Mark application as unavailable
		app.AvailabilityStatus = m.Unavailable
		app.AvailabilityStatusError = fmt.Sprintf("Failed to build superkey request: %v", err)
		appDao.Update(app)
		return err
	}

	// Create orchestrator
	orch := superkey.NewOrchestrator(sk.Tenant)
	ctx := context.Background()

	// Create resources
	result, err := orch.CreateResources(ctx, req)

	if err != nil {
		l.Log.Errorf("Failed to create superkey resources: %v", err)
		// Update application as unavailable
		orch.UpdateApplicationOnError(ctx, sk.ApplicationID, sk.Tenant, err)
		return err
	}

	// Update application with result
	err = orch.UpdateApplicationWithResult(ctx, sk.ApplicationID, result, req.Extra["result_type"])
	if err != nil {
		l.Log.Errorf("Failed to update application with result: %v", err)
		return err
	}

	// Create authentication
	err = orch.CreateAuthentication(ctx, sk.ApplicationID, sk.Tenant, result)
	if err != nil {
		l.Log.Errorf("Failed to create authentication: %v", err)
		return err
	}

	// Raise the Application.create event
	service.RaiseEvent("Application.create", app, sk.Headers)

	l.Log.Infof("Successfully completed superkey creation for application %d", sk.ApplicationID)
	return nil
}

func (sk SuperkeyCreateJob) ToJSON() []byte {
	bytes, err := json.Marshal(&sk)
	if err != nil {
		panic(err)
	}

	return bytes
}
