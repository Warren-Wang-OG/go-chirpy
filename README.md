# Chirpy Web Server
RESTful API

## Available Endpoints:

`POST /api/users` - Create a new User

`PUT /api/users` - Update an existing User, need to be authenticated already

`POST /api/login` - Authenticate a User / login

`POST /api/polka/webhooks` - Upgrade a user to "Chirpy Red" 

`POST /api/refresh` - Use a refresh token to get a new access token

`POST /api/revoke` - Revoke the given refresh token

`POST /api/chirps` - Create a Chirp (post), authenticated endpoint

`GET /api/chirps` - Get all chirps

`GET /api/chirps{id}` - Get a single Chirp by its `id`

`DELETE /api/chirps/{chirpID}` - Delete a chirp by its `id`, authenticated endpoint

`GET /api/healthz` - Readiness Endpoint

`GET /admin/metrics` - Get how many times `/` has been served

## Other files

`/assets/logo.png` - a random png logo


Notes:
- 
- "Chirpy Red" is a fictitious elevated subscription tier that users get upgraded to from 


*Project written following the outlines of "Learn Web Servers" on [boot.dev](https://boot.dev/tracks/backend)*