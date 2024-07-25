class Game extends HTMLElement {
  constructor() {
    super();
    this.movement = { up: false, down: false, left: false, right: false };

    this.handle_keypress = this.handle_keypress.bind(this);
    this.handle_message = this.handle_message.bind(this);
    this.animate = this.animate.bind(this);
    this.sequence = 1;
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
    }

    // TODO: Handle errors appropriately
    if (data.type === "error") {
      console.log(data);
      this.ws.close();
    }
  }

  animate() {
    // TODO: Slow this down, sends too frequently
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
      this.sequence++;
    }

    this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);

    // TODO: Add predictive movement using sequence numbers
    if (this.players) {
      this.players.forEach((player) => {
        this.ctx.beginPath();
        this.ctx.arc(
          player.info.x * this.dpr,
          player.info.y * this.dpr,
          10 * this.dpr,
          0,
          Math.PI * 2,
          false,
        );
        this.ctx.fillStyle = "black";
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
    this.canvas.width = 768 * this.dpr;
    this.canvas.height = 768 * this.dpr;

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
