TEST_DIR = $(CURDIR)/test_execution

test: clean
	mkdir -p $(TEST_DIR)
	go test -cover -coverprofile=$(TEST_DIR)/coverage.out -race ./... -v -covermode atomic -args -test.gocoverdir="$(TEST_DIR)"
	go tool cover -html=$(TEST_DIR)/coverage.out -o $(TEST_DIR)/coverage.html

clean:
	rm -rf $(TEST_DIR)