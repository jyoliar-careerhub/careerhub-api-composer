package controller

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/api"
	"github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/common/user_authority"
	scrapGrpc "github.com/jae2274/careerhub-api-composer/careerhub/apicomposer/userinfo/restapi_grpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func matchJobToken(t *testing.T) string {
	return newToken(t, "match-user", []string{user_authority.AuthorityMatchJob})
}

func TestMatchJobStrict_Authority(t *testing.T) {
	// AC: CheckHasAuthority 권한 검사가 전환 후에도 동작한다 (401/403 경로 · 본문 없음 유지)
	cases := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{name: "비로그인 → 401", token: "", wantStatus: http.StatusUnauthorized},
		{name: "권한 없는 토큰 → 403", token: "no-authority", wantStatus: http.StatusForbidden},
	}

	endpoints := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/api/my/match-job", nil},
		{http.MethodPost, "/api/my/match-job/condition", map[string]any{"conditionName": "c", "query": map[string]any{}}},
		{http.MethodPut, "/api/my/match-job/condition", map[string]any{"conditionId": "1", "conditionName": "c", "query": map[string]any{}}},
		{http.MethodDelete, "/api/my/match-job/condition", map[string]any{"conditionId": "1"}},
		{http.MethodPut, "/api/my/match-job/agree-to-mail", map[string]any{"agreeToMail": true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakes()
			server, _ := newStrictTestServer(t, f)

			token := ""
			if tc.token == "no-authority" {
				token = newToken(t, "user-1", []string{"AUTHORITY_READ_REVIEW"})
			}

			for _, ep := range endpoints {
				resp, body := doRequest(t, ep.method, server.URL+ep.path, token, ep.body)
				assert.Equal(t, tc.wantStatus, resp.StatusCode, "%s %s — body: %s", ep.method, ep.path, body)
				assert.Empty(t, body, "%s %s — 본문 없음 유지", ep.method, ep.path)
			}
		})
	}
}

func TestMatchJobStrict_FindMatchJob(t *testing.T) {
	f := newFakes()
	minCareer := int32(3)
	f.matchJob.findRes = &scrapGrpc.FindMatchJobResponse{
		AgreeToMail: true,
		Conditions: []*scrapGrpc.Condition{{
			ConditionId:   "cond-1",
			ConditionName: "백엔드 3년+",
			Query: &scrapGrpc.Query{
				Categories: []*scrapGrpc.Category{{Site: "wanted", CategoryName: "백엔드"}},
				SkillNames: []*scrapGrpc.Skill{{Or: []string{"go", "kotlin"}}},
				MinCareer:  &minCareer,
			},
		}},
	}
	server, _ := newStrictTestServer(t, f)

	resp, body := doRequest(t, http.MethodGet, server.URL+"/api/my/match-job", matchJobToken(t), nil)

	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", body)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")

	var matchJob api.MatchJob
	require.NoError(t, json.Unmarshal(body, &matchJob))
	assert.True(t, matchJob.AgreeToMail)
	require.Len(t, matchJob.Conditions, 1)
	cond := matchJob.Conditions[0]
	assert.Equal(t, "cond-1", cond.ConditionId)
	require.NotNil(t, cond.Query.Categories)
	assert.Equal(t, "백엔드", (*cond.Query.Categories)[0].CategoryName)
	require.NotNil(t, cond.Query.SkillNames)
	assert.Equal(t, [][]string{{"go", "kotlin"}}, *cond.Query.SkillNames)
	require.NotNil(t, cond.Query.MinCareer)
	assert.Equal(t, int32(3), *cond.Query.MinCareer)
	// maxCareer 미설정 → 필드 생략 (명세 옵셔널 — 기존 null에서의 수렴, 티켓 코멘트 기록)
	assert.Nil(t, cond.Query.MaxCareer)
}

func TestMatchJobStrict_Mutations(t *testing.T) {
	t.Run("조건 추가 — limitCount=2·토큰 userId 전달, isSuccess 반환", func(t *testing.T) {
		f := newFakes()
		f.matchJob.isSuccess = true
		server, _ := newStrictTestServer(t, f)

		resp, body := doRequest(t, http.MethodPost, server.URL+"/api/my/match-job/condition", matchJobToken(t),
			map[string]any{"conditionName": "프론트", "query": map[string]any{"skillNames": [][]string{{"svelte"}}, "minCareer": 1}})

		require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", body)
		assert.JSONEq(t, `{"isSuccess": true}`, string(body))
		require.NotNil(t, f.matchJob.gotAdd)
		assert.Equal(t, "match-user", f.matchJob.gotAdd.UserId)
		assert.Equal(t, uint32(2), f.matchJob.gotAdd.LimitCount)
		assert.Equal(t, "프론트", f.matchJob.gotAdd.Condition.ConditionName)
		require.Len(t, f.matchJob.gotAdd.Condition.Query.SkillNames, 1)
		assert.Equal(t, []string{"svelte"}, f.matchJob.gotAdd.Condition.Query.SkillNames[0].Or)
	})

	t.Run("조건 삭제 — 본문 conditionId 전달", func(t *testing.T) {
		f := newFakes()
		f.matchJob.isSuccess = true
		server, _ := newStrictTestServer(t, f)

		resp, body := doRequest(t, http.MethodDelete, server.URL+"/api/my/match-job/condition", matchJobToken(t),
			map[string]any{"conditionId": "cond-9"})

		require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", body)
		require.NotNil(t, f.matchJob.gotDelete)
		assert.Equal(t, "cond-9", f.matchJob.gotDelete.ConditionId)
	})

	t.Run("메일 수신 동의 변경", func(t *testing.T) {
		f := newFakes()
		f.matchJob.isSuccess = true
		server, _ := newStrictTestServer(t, f)

		resp, body := doRequest(t, http.MethodPut, server.URL+"/api/my/match-job/agree-to-mail", matchJobToken(t),
			map[string]any{"agreeToMail": false})

		require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", body)
		assert.JSONEq(t, `{"isSuccess": true}`, string(body))
		require.NotNil(t, f.matchJob.gotAgree)
		assert.False(t, f.matchJob.gotAgree.AgreeToMail)
	})
}
