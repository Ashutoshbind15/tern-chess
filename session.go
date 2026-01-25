package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
)

type SessionManager struct {
	fingerPrintToProgram map[string]*tea.Program
}

func (s *SessionManager) SetProgram(fingerPrint string, program *tea.Program) {
	log.Info("Setting program for fingerprint", "fingerprint", fingerPrint)
	s.fingerPrintToProgram[fingerPrint] = program
}

func (s *SessionManager) GetProgram(fingerPrint string) *tea.Program {
	return s.fingerPrintToProgram[fingerPrint]
}

func (s *SessionManager) RemoveProgram(fingerPrint string) {
	delete(s.fingerPrintToProgram, fingerPrint)
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		fingerPrintToProgram: make(map[string]*tea.Program),
	}
}