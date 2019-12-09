echo "Cleaning old logs directory"
rm -rf /Users/olsynt/workspace/personal/trello-sync/logs/* || true
echo "Building binary"
go build -o trello-sync .
echo "Running..."
./trello-sync sync --logs ./logs --trello-app-key $TRELLO_APP_KEY --trello-token $TRELLO_TOKEN --trello-board-id $TRELLO_BOARD_ID --store ./logs/store.yaml --google-service-account $GOOGLE_SERVICE_ACCOUNT_PATH --google-spreadsheet-id $GOOGLE_SPREADSHEET_ID --trello-service $TRELLO_SERVICE_PATH --google-spreadsheet-service $GOOGLE_SERVICE_PATH