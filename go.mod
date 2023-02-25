module github.com/tomlazar/football-threads

go 1.17

require (
	github.com/bwmarrin/discordgo v0.23.3-0.20210821175000-0fad116c6c2a
	github.com/caarlos0/env v3.5.0+incompatible
	github.com/joho/godotenv v1.3.0
	github.com/matryer/is v1.4.0
	github.com/pkg/errors v0.9.1
	github.com/rs/zerolog v1.25.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/stretchr/testify v1.7.0 // indirect
	golang.org/x/crypto v0.1.0 // indirect
	golang.org/x/sys v0.1.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

replace github.com/bwmarrin/discordgo v0.23.3-0.20210821175000-0fad116c6c2a => ../discordgo
