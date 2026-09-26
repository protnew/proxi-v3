package main

var engageTunnel func(endpoint string, selfExit bool)
var releaseTunnel func()
var activeState *helperState
