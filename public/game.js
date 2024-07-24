class Game extends HTMLElement {
	constructor() {
		super();
		this.shadow = this.attachShadow({ mode: 'open' });
	}

	join(event) {
		let buttons = this.shadow.querySelector("button")
		buttons.removeEventListener("click", this.join)

		console.log(event.target);
	}

	async connectedCallback() {
		let data = await fetch('/room/list').then(async r => {
			return await r.json().then(data => {return data});
		})

		data.forEach((room) => {
			this.shadowRoot.innerHTML += `
			<style>
				.room {
					display: flex;
					flex-direction: row;
					justify-content: space-between;
				}
			</style>
			<div class="room">
				<p>${room.name}</p>
				<button id="${room.id}">Connect</button>
			</div>
			`
		})

		let buttons = this.shadow.querySelector("button")
		buttons.addEventListener("click", this.join)

		/* Canvas
		this.shadowRoot.innerHTML = `
          <canvas id="game-window"></canvas>
        `;
		 */
	}

	disconnectedCallback() {
		let buttons = this.shadow.querySelector("button")
		buttons.removeEventListener("click", this.join)
		this.shadowRoot.innerHTML = '';
		console.log('Game destroyed');
	}
}

customElements.define('game-window', Game);