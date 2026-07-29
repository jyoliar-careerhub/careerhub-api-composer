BINARY_NAME=myapp
CODE_DIR=./careerhub/apicomposer
CONTAINER_IMAGE_NAME=careerhub-api-composer

# test.env는 로컬 전용(untracked)이라 없을 수 있다 — 없으면 조용히 건너뛴다 (run/test 외 타겟이 죽지 않도록)
-include test.env

## build: Build binary
build:
	@echo "Building..."
	@go build -ldflags="-s -w" -o ${BINARY_NAME} ${CODE_DIR}
	@echo "Built!"

image_build:
	@echo "Building..."
	@docker build -t ${CONTAINER_IMAGE_NAME}:latest .
	@echo "Built!"

## run: builds and runs the application
run: build
	@echo "Starting..."
	@env POSTING_GRPC_ENDPOINT=${POSTING_GRPC_ENDPOINT} API_PORT=${API_PORT} SECRET_KEY=${SECRET_KEY} REVIEW_GRPC_ENDPOINT=${REVIEW_GRPC_ENDPOINT} ./${BINARY_NAME} 
	@echo "Started!"

## clean: runs go clean and deletes binaries
clean:
	@echo "Cleaning..."
	@go clean
	@rm ${BINARY_NAME}
	@echo "Cleaned!"

## start: an alias to run
start: run

## stop: stops the running application
stop:
	@echo "Stopping..."
	@-pkill -SIGTERM -f "./${BINARY_NAME}"
	@echo "Stopped!"

## restart: stops and starts the application
restart: stop start

proto:
	@protoc careerhub/apicomposer/posting/restapi_grpc/*.proto  --go_out=. --go-grpc_out=. --go-grpc_opt=paths=source_relative  --go_opt=paths=source_relative  --proto_path=.
	@protoc careerhub/apicomposer/userinfo/restapi_grpc/*.proto  --go_out=. --go-grpc_out=. --go-grpc_opt=paths=source_relative  --go_opt=paths=source_relative  --proto_path=.
	@protoc careerhub/apicomposer/review/restapi_grpc/*.proto  --go_out=. --go-grpc_out=. --go-grpc_opt=paths=source_relative  --go_opt=paths=source_relative  --proto_path=.

## openapi: regenerates Go models & server interfaces from the OpenAPI spec
openapi:
	@cd careerhub/apicomposer/api && go tool oapi-codegen --config oapi-codegen.yaml openapi.yaml

## test: runs all tests
# fake 기반 테스트라 실 업스트림이 불필요하다 — test.env가 없으면 더미 기본값으로 돈다 (CI 포함)
# .PHONY 없으면 test/ 디렉터리를 타겟 산출물로 오인해 스킵된다
.PHONY: test
test: POSTING_GRPC_ENDPOINT ?= localhost:1
test: USERINFO_GRPC_ENDPOINT ?= localhost:1
test: REVIEW_GRPC_ENDPOINT ?= localhost:1
test: SECRET_KEY ?= test-secret-key
test: API_PORT ?= 18080
test:
	@echo "Testing..."
	@env POSTING_GRPC_ENDPOINT=${POSTING_GRPC_ENDPOINT} USERINFO_GRPC_ENDPOINT=${USERINFO_GRPC_ENDPOINT} API_PORT=${API_PORT} SECRET_KEY=${SECRET_KEY} REVIEW_GRPC_ENDPOINT=${REVIEW_GRPC_ENDPOINT} go test -p 1 -timeout 60s ./...
	

