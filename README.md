<div align="left">
  
![GitHub Actions](https://img.shields.io/badge/github%20actions-%232671E5.svg?style=for-the-badge&logo=githubactions&logoColor=white)
[![CI](https://github.com/docfrench/diceroller/actions/workflows/ci.yml/badge.svg)](https://github.com/docfrench/diceroller/actions/workflows/ci.yml) 
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/docfrench/diceroller)

</div>



# diceroller

Diceroller is a single page app written in Golang. Rolls dice for the Storyteller ttRPG system, with a live-streamed roll history between users.

## Features

- Deploys as a single Docker container with no additional dependencies
- Saves roll history in a local sqlite file
- Written in Golang
    - Uses Go + Templ <a href="https://pkg.go.dev/github.com/a-h/templ"><img src="https://pkg.go.dev/badge/github.com/a-h/templ.svg" alt="Go Reference" /></a> + <a href="https://github.com/bigskysoftware/htmx">HTMX</a> to create a responsive webform to roll dice
    - Uses Go hub/channels to provide a live-updated stream of dice rolls (displayed roll are also backfilled from the history on page load)

## Use

Complete the following fields and click Roll

- Character Name (optional)
- Reason (optional)
- Dice, in format 5d10
- Difficulty
- Table passphrase

On clicking Roll, the app will roll the requested dice, sort them, and tell the user how many successes they earned. The roll events are recorded in a persistent sqlite database for future review.

Although Character Name and Reason are optional, it is recommended to complete these fields to help with parsing the logs after the fact. Additionally, it is planned to automate replication of the roll data into the database for the actively-running campaign hosted by <a href="https://github.com/docfrench/deimos_api">this API</a>. The goal is for roll history to be associated with the rest of the character data hosted there.

Table passphrase is a simple 'gate check' to prevent webcrawlers from rolling dice.
