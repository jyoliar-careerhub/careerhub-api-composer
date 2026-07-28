package posting

import (
	"fmt"

	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/common/query"
)

func IsValid(queryReq *query.Query) error {
	if queryReq.MinCareer != nil && *queryReq.MinCareer < 0 {
		return fmt.Errorf("invalid minCareer value. %d", *queryReq.MinCareer)
	}

	if queryReq.MaxCareer != nil && *queryReq.MaxCareer < 0 {
		return fmt.Errorf("invalid maxCareer value. %d", *queryReq.MaxCareer)
	}

	if queryReq.MinCareer != nil && queryReq.MaxCareer != nil && *queryReq.MinCareer > *queryReq.MaxCareer {
		return fmt.Errorf("invalid minCareer and maxCareer value. minCareer(%d) > maxCareer(%d)", *queryReq.MinCareer, *queryReq.MaxCareer)
	}

	return nil
}
