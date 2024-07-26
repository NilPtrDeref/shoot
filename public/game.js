class Game extends HTMLElement {
  constructor() {
    super();
    this.width = 768;
    this.height = 768;
    this.player_radius = 10;
    this.move_delta = 5;
    this.movement = { up: false, down: false, left: false, right: false };
    this.players = [];
    this.queued_movements = [];
    this.sequence = 1;

    this.handle_keypress = this.handle_keypress.bind(this);
    this.handle_message = this.handle_message.bind(this);
    this.animate = this.animate.bind(this);
    this.process_movement = this.process_movement.bind(this);
    this.replay_movements_from_sequences =
      this.replay_movements_from_sequences.bind(this);
  }

  handle_keypress(event) {
    switch (event.key) {
      case "w" || "ArrowUp":
        this.movement.up = event.type === "keydown";
        break;
      case "s" || "ArrowDown":
        this.movement.down = event.type === "keydown";
        break;
      case "a" || "ArrowLeft":
        this.movement.left = event.type === "keydown";
        break;
      case "d" || "ArrowRight":
        this.movement.right = event.type === "keydown";
        break;
    }
  }

  handle_message(message) {
    let data = JSON.parse(message.data);

    if (data.type === "bootstrap") {
      this.self = data.id;
      this.players = data.players;
    }

    if (data.type === "update") {
      this.players = data.players;

      // Every time we update the players positions, recalculate the predicted position based on
      // sequence numbers of queued movements
      let player = this.players.find((p) => {
        return p.id === this.self;
      });
      if (player) {
        this.replay_movements_from_sequences(player);
      }
    }

    // TODO: Handle errors appropriately
    if (data.type === "error") {
      console.log(data);
      this.ws.close();
    }
  }

  replay_movements_from_sequences(player) {
    while (this.queued_movements.length > 0) {
      let queued = this.queued_movements.shift();
      if (player.info.sequence < queued.sequence) {
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
      if (player.info.y >= this.player_radius + this.move_delta) {
        player.info.y -= this.move_delta;
      } else {
        player.info.y = 0 + this.player_radius;
      }
    }
    if (movement.down) {
      if (
        player.info.y <=
        this.height - (this.player_radius + this.move_delta)
      ) {
        player.info.y += this.move_delta;
      } else {
        player.info.y = this.height - this.player_radius;
      }
    }
    if (movement.left) {
      if (player.info.x >= this.player_radius + this.move_delta) {
        player.info.x -= this.move_delta;
      } else {
        player.info.x = 0 + this.player_radius;
      }
    }
    if (movement.right) {
      if (
        player.info.x <=
        this.width - (this.player_radius + this.move_delta)
      ) {
        player.info.x += this.move_delta;
      } else {
        player.info.x = this.width - this.player_radius;
      }
    }
  }

  animate() {
    // TODO: Slow this down, sends too frequently, try to bring down to 10 times per second
    if (
      this.movement.up ||
      this.movement.down ||
      this.movement.left ||
      this.movement.right
    ) {
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
      let player = this.players.find((p) => {
        return p.id === this.self;
      });
      if (player) {
        this.process_movement(player, this.movement);
      }

      this.sequence++;
    }

    this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);

    if (this.players) {
      this.players.forEach((player) => {
        this.ctx.beginPath();
        this.ctx.arc(
          player.info.x * this.dpr,
          player.info.y * this.dpr,
          this.player_radius * this.dpr,
          0,
          Math.PI * 2,
          false,
        );
        this.ctx.fillStyle = "white";
        this.ctx.fill();
        this.ctx.closePath();
      });
    }

    requestAnimationFrame(this.animate);
  }

  connectedCallback() {
    this.attachShadow({ mode: "open" });

    this.shadowRoot.innerHTML = `
      <link rel="stylesheet" href="/public/global.css">
      <canvas id="game-window"></canvas>
    `;

    this.dpr = window.devicePixelRatio ?? 1;
    this.canvas = this.shadowRoot.getElementById("game-window");
    // TODO: Handle error when canvas can't be found
    if (!this.canvas) {
      console.error("failed to load canvas");
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
    this.animate();
  }

  disconnectedCallback() {
    window.removeEventListener("keydown", this.handle_keypress);
    window.removeEventListener("keyup", this.handle_keypress);

    this.ws.close();
    this.shadowRoot.innerHTML = "";
  }
}

customElements.define("game-window", Game);
