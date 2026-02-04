const connectionStatus = document.getElementById("connection-status");
const latestBlock = document.getElementById("latest-block");
const latestHash = document.getElementById("latest-hash");
const blockFeed = document.getElementById("block-feed");
const tpsElement = document.getElementById("tps");
const validatorsElement = document.getElementById("validators");
const epochElement = document.getElementById("epoch");
const walletButton = document.getElementById("wallet-button");
const walletStatus = document.getElementById("wallet-status");
const stakeForm = document.getElementById("stake-form");
const stakeAmount = document.getElementById("stake-amount");
const validatorSelect = document.getElementById("validator-select");
const stakeResult = document.getElementById("stake-result");

const MAX_FEED_ITEMS = 8;
let currentBlock = 982341;
let socket;
let walletConnected = false;

const formatHash = (hash) => `${hash.slice(0, 10)}...${hash.slice(-6)}`;

const renderBlock = (block) => {
  const item = document.createElement("li");
  item.innerHTML = `
    <div class="block-meta">
      <strong>Block #${block.height}</strong>
      <span>${formatHash(block.hash)}</span>
    </div>
    <span>${block.txCount} txns • ${block.leader}</span>
  `;

  blockFeed.prepend(item);
  if (blockFeed.children.length > MAX_FEED_ITEMS) {
    blockFeed.removeChild(blockFeed.lastElementChild);
  }
};

const updateHero = (block) => {
  latestBlock.textContent = `#${block.height}`;
  latestHash.textContent = formatHash(block.hash);
};

const updateMetrics = () => {
  const tps = (4200 + Math.random() * 1800).toFixed(0);
  const validators = (128 + Math.random() * 32).toFixed(0);
  const epoch = (42 + Math.random() * 40).toFixed(1);
  tpsElement.textContent = tps;
  validatorsElement.textContent = validators;
  epochElement.textContent = `${epoch}%`;
};

const generateBlock = () => {
  currentBlock += Math.floor(Math.random() * 4) + 1;
  const hash = crypto.randomUUID().replace(/-/g, "");
  return {
    height: currentBlock,
    hash,
    txCount: Math.floor(Math.random() * 500 + 1200),
    leader: ["Helios", "Nova", "Lumen", "Pulse", "Vertex"][
      Math.floor(Math.random() * 5)
    ],
  };
};

const startMockStream = () => {
  connectionStatus.textContent = "Streaming (simulated)";
  setInterval(() => {
    const block = generateBlock();
    renderBlock(block);
    updateHero(block);
  }, 1800);
};

const connectWebSocket = () => {
  try {
    socket = new WebSocket("ws://localhost:8080/blocks");
  } catch (error) {
    startMockStream();
    return;
  }

  socket.addEventListener("open", () => {
    connectionStatus.textContent = "WebSocket connected";
  });

  socket.addEventListener("message", (event) => {
    try {
      const block = JSON.parse(event.data);
      renderBlock(block);
      updateHero(block);
    } catch (error) {
      const block = generateBlock();
      renderBlock(block);
      updateHero(block);
    }
  });

  socket.addEventListener("close", () => {
    connectionStatus.textContent = "Connection lost — using fallback";
    startMockStream();
  });

  socket.addEventListener("error", () => {
    connectionStatus.textContent = "WebSocket error — using fallback";
    startMockStream();
  });
};

const initTabs = () => {
  const buttons = document.querySelectorAll(".tab-button");
  const contents = document.querySelectorAll(".tab-content");

  buttons.forEach((button) => {
    button.addEventListener("click", () => {
      buttons.forEach((btn) => btn.classList.remove("active"));
      contents.forEach((content) => content.classList.remove("active"));
      button.classList.add("active");
      document.getElementById(button.dataset.tab).classList.add("active");
    });
  });
};

walletButton.addEventListener("click", () => {
  walletConnected = !walletConnected;
  if (walletConnected) {
    walletStatus.textContent = "Wallet connected: 9xDk...e7A";
    walletButton.textContent = "Disconnect";
  } else {
    walletStatus.textContent = "Not connected";
    walletButton.textContent = "Connect Wallet";
  }
});

stakeForm.addEventListener("submit", (event) => {
  event.preventDefault();
  if (!walletConnected) {
    stakeResult.textContent = "Connect your wallet before staking.";
    return;
  }

  const amount = Number(stakeAmount.value);
  if (!amount || amount <= 0) {
    stakeResult.textContent = "Enter a valid amount of SUNC.";
    return;
  }

  stakeResult.textContent = `Staked ${amount} SUNC to ${validatorSelect.options[
    validatorSelect.selectedIndex
  ].text}.`;
  stakeAmount.value = "";
});

updateMetrics();
setInterval(updateMetrics, 2000);
connectWebSocket();
initTabs();
