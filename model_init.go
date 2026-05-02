package main

import "github.com/Ashutoshbind15/ssh-chess/common"

func initModel(fingerPrint string) model {
	chatTa := common.InitTextArea()
	usernameInputTa := common.InitTextInput()
	gameJoinInput := common.InitTextInput()
	moveInput := common.InitTextInput()
	gameJoinInput.Prompt = "game id> "
	gameJoinInput.Placeholder = "Enter game ID"
	moveInput.Prompt = "move> "
	moveInput.Placeholder = "e2e4"

	player := dataManager.GetPlayer(fingerPrint)
	if player != nil {
		gameManager.SetPlayer(fingerPrint, player.Username)
	}

	return model{
		counter:       0,
		messages:      []message{},
		fingerPrint:   fingerPrint,
		chatTextarea:  chatTa,
		usernameInput: usernameInputTa,
		gameJoinInput: gameJoinInput,
		moveInput:     moveInput,
		page:          PageIntro,
		player:        player,
		pageList:      newPageList(80, 22),
		currentGame:   gameManager.GameForPlayer(fingerPrint),
	}
}
