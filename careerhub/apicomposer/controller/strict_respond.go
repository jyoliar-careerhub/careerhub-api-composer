package controller

import "github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/api"

// 상태코드별 에러 응답 생성자.
//
// 생성 코드는 연산×상태코드마다 별개 타입(예: GetJobPostings400JSONResponse)을 내는데,
// 공유 $ref 응답이라 리터럴이 이중이 되고 상태코드가 긴 이름 안에 묻힌다. 이 생성자들은
// 줄 머리에 상태 의미(badRequest400·notFound404·…)가 오도록 호출부를 정리한다.
//
// 구속은 그대로다: 타입 파라미터가 연산별 생성 타입이라 잘못된 연산 타입은 컴파일 실패하고,
// 구조 제약(~struct{...})이 상태-생성자 불일치(예: badRequest400에 500 타입)도 컴파일로 잡는다.

func badRequest400[T ~struct{ api.BadRequestJSONResponse }](message string) T {
	return T{api.BadRequestJSONResponse{Message: message}}
}

func unauthorized401[T ~struct{ api.UnauthorizedJSONResponse }]() T {
	return T{api.UnauthorizedJSONResponse{Message: messageUnauthorized}}
}

func notFound404[T ~struct{ api.NotFoundJSONResponse }](message string) T {
	return T{api.NotFoundJSONResponse{Message: message}}
}

func internalError500[T ~struct {
	api.InternalServerErrorJSONResponse
}]() T {
	return T{api.InternalServerErrorJSONResponse{Message: messageInternalServerError}}
}
