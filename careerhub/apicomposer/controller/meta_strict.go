package controller

import (
	"context"

	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/api"
)

// MetaStrictController는 meta 태그 연산(계약·서버 메타 정보)을 서빙한다.
//
// GetContractVersion은 합의 선행 머지(스펙+stub) 관행의 첫 사례다(ADR-0019,
// jyoliar-lab/ops-workflow#30): 계약은 공표됐고 구현은 아직이다 — 구현 완료까지
// 501 + ErrorResponse(명세의 NotImplemented 응답)로 명시적 미구현 신호를 보낸다.
// stub을 실 구현으로 바꾸는 PR의 요건은 README '합의 선행 머지' 절에 있다.
type MetaStrictController struct{}

func NewMetaStrictController() *MetaStrictController {
	return &MetaStrictController{}
}

func (c *MetaStrictController) GetContractVersion(ctx context.Context, request api.GetContractVersionRequestObject) (api.GetContractVersionResponseObject, error) {
	return notImplemented501[api.GetContractVersion501JSONResponse](), nil
}
