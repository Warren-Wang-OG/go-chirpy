# Chirpy Web Server
RESTful API

## Available Endpoints:

`POST /api/users` - Create a new User

`PUT /api/users` - Update an existing User, need to be authenticated already

`POST /api/login` - Authenticate a User / login

`POST /api/chirps` - Create a Chirp (post)

`GET /api/chirps` - Get all chirps

`GET /api/chirps{id}` - Get a single Chirp by its `id`

`GET /api/healthz` - Readiness Endpoint

`GET /admin/metrics` - Get how many times `/` has been served

## Other files

`/assets/logo.png` - a random png logo