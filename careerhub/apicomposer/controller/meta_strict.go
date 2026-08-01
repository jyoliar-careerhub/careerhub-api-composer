package controller

import (
	"context"

	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/api"
)

// MetaStrictController는 meta 태그 연산(계약·서버 메타 정보)을 서빙한다.
type MetaStrictController struct {
	contractVersion string
}

// NewMetaStrictController는 내장 명세에서 계약 버전을 한 번만 읽어 보관한다 —
// GetSpec()은 호출마다 내장 명세를 디코드하므로 요청 경로에 두지 않는다.
// 디코드 실패는 배선 시점에 끊는다(NewSpecValidationMW와 같은 취급).
func NewMetaStrictController() (*MetaStrictController, error) {
	swagger, err := api.GetSpec()
	if err != nil {
		return nil, err
	}
	return &MetaStrictController{contractVersion: swagger.Info.Version}, nil
}

func (c *MetaStrictController) GetContractVersion(ctx context.Context, request api.GetContractVersionRequestObject) (api.GetContractVersionResponseObject, error) {
	return api.GetContractVersion200JSONResponse{Version: c.contractVersion}, nil
}
