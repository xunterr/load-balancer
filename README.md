# load-balancer
---

This repository contains a lightweight and straightforward load balancer implementation that uses the round-robin algorithm. The load balancer is designed to evenly distribute incoming traffic across multiple backend servers.

## Features

- **Round-Robin Algorithm:** Requests are evenly distributed across backend servers.
- **Healthchecks:** Load balancer uses both passive and active healthchecks. Active one runs every minute.

## Configuration

Use command line arguments to pass server addresses. Separate them with comas

Example: `./lb -s http://localhost:8081,http://localhost:8082`


## Todo:

- [ ] Integrate service discovery
- [ ] Add json configuration

## License

This project is licensed under the MIT License.
