package main

import "github.com/Ashutoshbind15/ssh-chess/common"

func initModel(fingerPrint string) model {
	chatTa := common.InitTextArea()
	usernameInputTa := common.InitTextInput()
	gameJoinInput := common.InitTextInput()
	gameJoinInput.Prompt = "game id> "
	gameJoinInput.Placeholder = "Enter game ID"

	player := dataManager.GetPlayer(fingerPrint)
	if player != nil {
		gameManager.SetPlayer(fingerPrint, player.Username)
	}

	gameState := gameManager.GetPlayerGameState(fingerPrint)

	return model{
		counter:       0,
		messages:      []message{},
		fingerPrint:   fingerPrint,
		chatTextarea:  chatTa,
		usernameInput: usernameInputTa,
		gameJoinInput: gameJoinInput,
		page:          PageIntro,
		player:        player,
		pageList:      newPageList(80, 22),
		currentGameID: gameState.CurrentGameID,
		gameStatus:    gameState.GameStatus,
	}
}
