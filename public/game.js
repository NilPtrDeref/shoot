function throttle(func, limit) {
  let inThrottle;
  return function (...args) {
    if (!inThrottle) {
      func.apply(this, args);
      inThrottle = true;
      setTimeout(() => (inThrottle = false), limit);
    }
  };
}

function normalizeVector(x1, y1, x2, y2) {
  // Calculate the differences
  const deltaX = x2 - x1;
  const deltaY = y2 - y1;

  // Calculate the magnitude of the vector
  const magnitude = Math.sqrt(deltaX * deltaX + deltaY * deltaY);

  // Handle the case when the magnitude is zero to avoid division by zero
  if (magnitude === 0) {
    return { x: 0, y: 0 };
  }

  // Normalize the vector components
  const normalizedX = deltaX / magnitude;
  const normalizedY = deltaY / magnitude;

  return { x: normalizedX, y: normalizedY };
}
class Game extends HTMLElement {
  constructor() {
    super();
    this.width = 768;
    this.height = 768;
    this.player_radius = 10;
    this.bullet_radius = 4;
    this.move_delta = 5;
    this.movement = { up: false, down: false, left: false, right: false };
    this.mouse = { clicked: false, x: 0, y: 0 };
    this.players = [];
    this.bullets = [];
    this.queued_movements = [];
    this.sequence = 1;
    this.quit = false;

    this.handle_keypress = this.handle_keypress.bind(this);
    this.handle_mouse = this.handle_mouse.bind(this);
    this.handle_message = this.handle_message.bind(this);
    this.update = this.update.bind(this);
    this.animate = this.animate.bind(this);
    this.detach = this.detach.bind(this);
    this.process_movement = this.process_movement.bind(this);
    this.trigger_movement = this.trigger_movement.bind(this);
    this.throttled_trigger_fire = throttle((player) => {
      this.trigger_fire(player);
    }, 200);
    this.replay_movements_from_sequences =
      this.replay_movements_from_sequences.bind(this);
  }

  handle_keypress(event) {
    switch (event.key) {
      case "w":
      case "ArrowUp":
        this.movement.up = event.type === "keydown";
        break;
      case "s":
      case "ArrowDown":
        this.movement.down = event.type === "keydown";
        break;
      case "a":
      case "ArrowLeft":
        this.movement.left = event.type === "keydown";
        break;
      case "d":
      case "ArrowRight":
        this.movement.right = event.type === "keydown";
        break;
      case "c":
        this.ws.send(
          JSON.stringify({
            type: "reskin",
          }),
        );
        break;
      case "Escape":
        location.reload();
        break;
    }
  }

  handle_mouse(event) {
    switch (event.type) {
      case "mouseup":
      case "mouseleave":
        this.mouse.clicked = false;
        break;
      case "mousedown":
        this.mouse.clicked = true;
        break;
      case "mousemove":
        this.mouse.x = event.offsetX;
        this.mouse.y = event.offsetY;
        break;
    }
  }

  handle_error(error) {
    this.quit = true;
    this.ws.close();
    this.detach();
    this.shadowRoot.innerHTML = `
      <link rel="stylesheet" href="/public/global.css">
      <h1 class="error">${error}</h1>
      <p class="error">Please refresh to go back to lobby</p>
    `;
  }

  handle_message(message) {
    let data = JSON.parse(message.data);

    if (data.type === "bootstrap") {
      this.self = data.id;
    }

    if (data.type === "update") {
      this.players = data.players;
      this.bullets = data.bullets;

      // Every time we update the players positions, recalculate the predicted position based on
      // sequence numbers of queued movements
      let player = this.players.find((p) => {
        return p.id === this.self;
      });
      if (player) {
        this.replay_movements_from_sequences(player);
      }
    }

    // Handle errors
    if (data.type === "error") {
      this.handle_error(data.error);
    }
  }

  replay_movements_from_sequences(player) {
    while (this.queued_movements.length > 0) {
      let queued = this.queued_movements.shift();
      if (player.sequence < queued.sequence) {
        this.queued_movements.unshift(queued);
        break;
      }
    }

    this.queued_movements.forEach((queued) => {
      this.process_movement(player, queued.movement);
    });
  }

  process_movement(player, movement) {
    if (movement.up) {
      if (player.position.y >= this.player_radius + this.move_delta) {
        player.position.y -= this.move_delta;
      } else {
        player.position.y = 0 + this.player_radius;
      }
    }
    if (movement.down) {
      if (
        player.position.y <=
        this.height - (this.player_radius + this.move_delta)
      ) {
        player.position.y += this.move_delta;
      } else {
        player.position.y = this.height - this.player_radius;
      }
    }
    if (movement.left) {
      if (player.position.x >= this.player_radius + this.move_delta) {
        player.position.x -= this.move_delta;
      } else {
        player.position.x = 0 + this.player_radius;
      }
    }
    if (movement.right) {
      if (
        player.position.x <=
        this.width - (this.player_radius + this.move_delta)
      ) {
        player.position.x += this.move_delta;
      } else {
        player.position.x = this.width - this.player_radius;
      }
    }
  }

  trigger_movement(player) {
    this.ws.send(
      JSON.stringify({
        type: "movement",
        sequence: this.sequence,
        movement: this.movement,
      }),
    );

    // Use predictive movement to compensate for lag
    this.queued_movements.push({
      sequence: this.sequence,
      movement: this.queued_movements,
    });
    this.process_movement(player, this.movement);

    this.sequence++;
  }

  trigger_fire(player) {
    this.ws.send(
      JSON.stringify({
        type: "fire",
        bullet: {
          owner: this.self,
          direction: normalizeVector(
            player.position.x,
            player.position.y,
            this.mouse.x,
            this.mouse.y,
          ),
        },
      }),
    );
  }

  update() {
    let player = this.players.find((p) => {
      return p.id === this.self;
    });

    // Handle player movement
    if (
      this.movement.up ||
      this.movement.down ||
      this.movement.left ||
      this.movement.right
    ) {
      this.trigger_movement(player);
    }

    // Handle trigger bullet creation events
    if (this.mouse.clicked) {
      this.throttled_trigger_fire(player);
    }
  }

  animate() {
    this.update();
    this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);

    if (this.players) {
      this.players.forEach((player) => {
        if (player.spawn_time > 0) return;

        this.ctx.beginPath();
        this.ctx.arc(
          player.position.x * this.dpr,
          player.position.y * this.dpr,
          this.player_radius * this.dpr,
          0,
          Math.PI * 2,
          false,
        );
        this.ctx.fillStyle = `hsl(${player.hue}, 100%, 50%)`;
        this.ctx.fill();
        this.ctx.closePath();
      });
    }

    if (this.bullets) {
      this.bullets.forEach((bullet) => {
        let player = this.players.find((p) => {
          return p.id === bullet.owner;
        });

        this.ctx.beginPath();
        this.ctx.arc(
          bullet.position.x * this.dpr,
          bullet.position.y * this.dpr,
          this.bullet_radius * this.dpr,
          0,
          Math.PI * 2,
          false,
        );
        this.ctx.fillStyle = `hsl(${player.hue}, 80%, 30%)`;
        this.ctx.fill();
        this.ctx.closePath();
      });
    }

    if (!this.quit) {
      requestAnimationFrame(this.animate);
    }
  }

  connectedCallback() {
    this.attachShadow({ mode: "open" });

    this.shadowRoot.innerHTML = `
      <link rel="stylesheet" href="/public/global.css">
      <canvas id="game-window"></canvas>
    `;

    this.dpr = window.devicePixelRatio ?? 1;
    this.canvas = this.shadowRoot.getElementById("game-window");
    if (!this.canvas) {
      this.handle_error("failed to load canvas");
      return;
    }
    this.width = this.width;
    this.height = this.height;
    this.move_delta = this.move_delta;
    this.player_radius = this.player_radius;

    this.canvas.width = this.width * this.dpr;
    this.canvas.height = this.height * this.dpr;

    this.ctx = this.canvas.getContext("2d");
    this.room = this.getAttribute("room");
    this.ws = new WebSocket(`/room/${this.room}/ws`);

    this.ws.addEventListener("message", this.handle_message);

    // TODO: On disconnect, try reconnect?
    // this.ws.addEventListener("close", () => {
    //   location.reload();
    // });

    window.addEventListener("keydown", this.handle_keypress);
    window.addEventListener("keyup", this.handle_keypress);
    this.canvas.addEventListener("mousedown", this.handle_mouse);
    this.canvas.addEventListener("mouseup", this.handle_mouse);
    this.canvas.addEventListener("mouseleave", this.handle_mouse);
    this.canvas.addEventListener("mousemove", this.handle_mouse);
    this.animate();
  }

  detach() {
    this.canvas.removeEventListener("mousedown", this.handle_mouse);
    this.canvas.removeEventListener("mouseup", this.handle_mouse);
    this.canvas.removeEventListener("mouseleave", this.handle_mouse);
    this.canvas.removeEventListener("mousemove", this.handle_mouse);
    window.removeEventListener("keydown", this.handle_keypress);
    window.removeEventListener("keyup", this.handle_keypress);
  }

  disconnectedCallback() {
    this.detach();

    this.ws.close();
    this.shadowRoot.innerHTML = "";
  }
}

customElements.define("game-window", Game);
