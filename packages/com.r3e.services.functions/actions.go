package functions

import (
	"context"
	"fmt"
)

// processActions processes devpack actions through the ActionProcessor.
func (s *Service) processActions(ctx context.Context, def Definition, actions []Action) ([]ActionResult, error) {
	results := make([]ActionResult, 0, len(actions))
	var firstErr error

	for _, action := range actions {
		res := ActionResult{
			Action: action,
			Status: ActionStatusSucceeded,
		}

		if s.actionProcessor == nil {
			res.Status = ActionStatusFailed
			res.Error = "action processor not configured"
		} else if !s.actionProcessor.SupportsAction(action.Type) {
			res.Status = ActionStatusFailed
			res.Error = fmt.Sprintf("unsupported action type %q", action.Type)
		} else {
			output, err := s.actionProcessor.ProcessAction(ctx, def.AccountID, action.Type, action.Params)
			if err != nil {
				res.Status = ActionStatusFailed
				res.Error = err.Error()
			} else {
				res.Result = output
			}
		}

		results = append(results, res)
		if res.Status == ActionStatusFailed && firstErr == nil {
			firstErr = fmt.Errorf("devpack action %s failed: %s", action.Type, res.Error)
		}
	}

	return results, firstErr
}
