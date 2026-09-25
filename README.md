<div align="left">
  
![GitHub Actions](https://img.shields.io/badge/github%20actions-%232671E5.svg?style=for-the-badge&logo=githubactions&logoColor=white)
[![CI](https://github.com/docfrench/diceroller/actions/workflows/ci.yml/badge.svg)](https://github.com/docfrench/diceroller/actions/workflows/ci.yml) 
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/docfrench/diceroller)

</div>



# diceroller

Diceroller is a single page app written in Golang. It rolls dice for common tabletop RPG systems, with a live-streamed roll history between users.

## Features

- Deploys as a single Docker container with no additional dependencies
- Saves roll history in a local sqlite file
- Written in Golang
    - Uses Go + Templ <a href="https://pkg.go.dev/github.com/a-h/templ"><img src="https://pkg.go.dev/badge/github.com/a-h/templ.svg" alt="Go Reference" /></a> + <a href="https://github.com/bigskysoftware/htmx">HTMX</a> to create a responsive webform to roll dice
    - Uses Go hub/channels to provide a live-updated stream of dice rolls (displayed roll are also backfilled from the history on page load)

## Use

Select the dice system from the dropdown menu. Once the fields appear, complete them and click Roll.
<div>
<img src="https://github.com/docfrench/diceroller/blob/main/images/result1.png" alt="results" width="400">
<img src="https://github.com/docfrench/diceroller/blob/main/images/result2.png" alt="results" width="400"></div>

On clicking Roll, the app will roll the requested dice and present the formatted results. The roll events are recorded in a persistent sqlite database; this is used to both backfill previous rolls upon page load, but also provides the option to attach roll history to characters in an RPG campaign.

<img src="https://github.com/docfrench/diceroller/blob/main/images/log.png" alt="logs" width="400">

Although Character Name and Reason are optional, it is recommended to complete these fields to help with parsing the logs in the future. Additionally, it is planned to automate replication of the roll data into the database for the actively-running campaign hosted by <a href="https://github.com/docfrench/deimos_api">this API</a>. The goal is for roll history to be associated with the rest of the character data hosted there.

Table passphrase is a simple 'gate check' to prevent webcrawlers from rolling dice.
