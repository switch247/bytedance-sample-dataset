

# before
docker compose run --rm --entrypoint="" before python3 -m pytest -q


# after
docker compose run --rm --entrypoint="" after go test ./... -v

# evaluation


docker compose run --rm evaluation
