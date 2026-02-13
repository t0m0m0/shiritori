      /* ========== CONSTANTS ========== */
      const DEFAULT_MAX_LIVES = 3;

      /* ========== STATE ========== */
      let ws = null;
      let myName = "";
      let currentRoomId = "";
      let currentSettings = {};
      let timerInterval = null;
      let timerMax = 0;
      let currentTurn = "";
      let turnOrder = [];
      let roomOwner = "";
      let waitingPlayers = [];
      let voteTimerInterval = null;
      let hasVoted = false;
      let isVoteActive = false;
      let lastWordPlayer = "";
      let wasInRoom = ""; // roomId to rejoin after reconnect
      let wasRoomOwner = false; // was this player the room owner?
      let kanaRowNames = [];
      let currentLives = {};
      let maxLives = DEFAULT_MAX_LIVES;
      let inviteRoomId = "";
      let lastShareURL = "";

      const $ = (id) => document.getElementById(id);

      /* ========== WEBSOCKET ========== */
      function connect() {
        if (ws && ws.readyState <= 1) return;
        const proto = location.protocol === "https:" ? "wss:" : "ws:";
        ws = new WebSocket(`${proto}//${location.host}/ws`);
        ws.onopen = () => {
          console.log("WS connected");
          refreshRooms();
          send({ type: "get_genres" });
          // Attempt to rejoin room after reconnect
          if (wasInRoom && myName) {
            console.log("Attempting to rejoin room", wasInRoom);
            send({ type: "join", name: myName, roomId: wasInRoom });
            wasInRoom = "";
          } else {
            document.body.classList.toggle(
              "invite-lobby",
              !!inviteRoomId && !currentRoomId,
            );
          }
        };
        ws.onclose = () => {
          console.log("WS closed");
          // Remember current room so we can rejoin
          if (currentRoomId) {
            wasInRoom = currentRoomId;
            wasRoomOwner = (roomOwner === myName);
          }
          setTimeout(connect, 2000);
        };
        ws.onerror = (e) => console.error("WS error", e);
        ws.onmessage = (e) => handleMessage(JSON.parse(e.data));
      }

      function getRoomLink(roomId) {
        const url = new URL(window.location.href);
        url.searchParams.set("room", roomId);
        return url.toString();
      }

      async function copyText(text) {
        try {
          await navigator.clipboard.writeText(text);
          return true;
        } catch (e) {
          const ta = document.createElement("textarea");
          ta.value = text;
          ta.style.position = "fixed";
          ta.style.opacity = "0";
          document.body.appendChild(ta);
          ta.select();
          try {
            const ok = document.execCommand("copy");
            document.body.removeChild(ta);
            return ok;
          } catch (err) {
            document.body.removeChild(ta);
            return false;
          }
        }
      }

      function setInviteRoom(roomId) {
        if (!roomId) return;
        inviteRoomId = roomId;
        $("inviteCardTitle").textContent = "招待ルーム";
        $("inviteCardRules").innerHTML =
          '<span class="rule-badge">読み込み中…</span>';
        $("inviteCardHost").textContent = "";
        $("inviteCard").classList.remove("hidden");
        document.body.classList.add("invite-lobby");
      }

      function clearInviteRoomInfo() {
        $("inviteCardTitle").textContent = "招待ルーム";
        $("inviteCardRules").innerHTML =
          '<span class="rule-badge">読み込み中…</span>';
        $("inviteCardHost").textContent = "";
      }

      function clearInvite() {
        inviteRoomId = "";
        const url = new URL(window.location.href);
        url.searchParams.delete("room");
        window.history.replaceState({}, "", url.toString());
        $("inviteCard").classList.add("hidden");
        clearInviteRoomInfo();
        document.body.classList.remove("invite-lobby");
      }

      function joinInviteRoom() {
        if (!inviteRoomId) return;
        joinRoom(inviteRoomId);
        $("inviteCard").classList.add("hidden");
        document.body.classList.remove("invite-lobby");
      }

      function copyRoomLink() {
        if (!currentRoomId) return;
        const link = getRoomLink(currentRoomId);
        copyText(link).then((ok) => {
          addMessage(
            ok
              ? "ルームのリンクをコピーしました"
              : "ルームのリンクをコピーできませんでした",
            ok ? "success" : "error",
          );
        });
      }

      function shareRoomToX() {
        if (!currentRoomId) return;
        const link = getRoomLink(currentRoomId);
        const title = $('gameRoomTitle') ? $('gameRoomTitle').textContent : 'ルーム';
        const text = `しりとりで遊ぼう！「${title}」に参加してね🎮\n`;
        const url = `https://x.com/intent/tweet?text=${encodeURIComponent(text)}&url=${encodeURIComponent(link)}`;
        window.open(url, '_blank', 'width=550,height=420');
      }

      function shareRoomToLINE() {
        if (!currentRoomId) return;
        const link = getRoomLink(currentRoomId);
        const title = $('gameRoomTitle') ? $('gameRoomTitle').textContent : 'ルーム';
        const text = `しりとりで遊ぼう！「${title}」に参加してね🎮\n${link}`;
        const url = `https://social-plugins.line.me/lineit/share?url=${encodeURIComponent(link)}&text=${encodeURIComponent(text)}`;
        window.open(url, '_blank', 'width=550,height=420');
      }


      function send(obj) {
        if (ws && ws.readyState === 1) ws.send(JSON.stringify(obj));
      }

      /* ========== MESSAGE HANDLER ========== */
      function handleMessage(msg) {
        switch (msg.type) {
          case "rooms":
            if (!inviteRoomId) renderRoomList(msg.rooms || []);
            break;
          case "genres":
            onGenres(msg);
            break;
          case "room_joined":
            onJoined(msg);
            break;
          case "player_joined":
            onPlayerJoined(msg.player);
            break;
          case "player_left":
            onPlayerLeft(msg.player);
            break;
          case "player_list":
            onPlayerList(msg);
            break;
          case "game_started":
            onGameStarted(msg);
            break;
          case "word_accepted":
            onNewWord(msg);
            break;
          case "answer_rejected":
            onInvalid(msg.message);
            break;
          case "timer":
            onTimer(msg.timeLeft);
            break;
          case "game_over":
            onGameOver(msg);
            break;
          case "vote_request":
            onVoteRequest(msg);
            break;
          case "vote_update":
            onVoteUpdate(msg);
            break;
          case "vote_result":
            onVoteResult(msg);
            break;
          case "rebuttal":
            onRebuttal(msg);
            break;
          case "challenge_withdrawn":
            onChallengeWithdrawn(msg);
            break;
          case "penalty":
            onPenalty(msg);
            break;
          case "turn_update":
            onTurnUpdate(msg);
            break;
          case "room_state":
            onJoined(msg);
            break;
          case "settings_updated":
            onSettingsUpdated(msg);
            break;
          case "error":
            addMessage(msg.message, "error");
            break;
          default:
            console.log("Unknown message", msg);
        }
      }

      /* ========== LOBBY ========== */
      function refreshRooms() {
        if (inviteRoomId && !currentRoomId) return;
        send({ type: "get_rooms" });
      }

      function onGenres(msg) {
        kanaRowNames = msg.kanaRows || [];
        renderKanaRowCheckboxes();
      }

      function renderKanaRowCheckboxes() {
        const container = $("kanaRowCheckboxes");
        if (!container || !kanaRowNames.length) return;
        container.innerHTML = "";
        kanaRowNames.forEach((name) => {
          const label = document.createElement("label");
          label.className = "kana-row-chip";
          label.innerHTML = `<input type="checkbox" value="${esc(name)}"> ${esc(name)}`;
          label.addEventListener("click", () => {
            setTimeout(() => {
              const cb = label.querySelector("input");
              label.classList.toggle("selected", cb.checked);
            }, 0);
          });
          container.appendChild(label);
        });
      }

      function getSelectedKanaRows() {
        const checkboxes = document.querySelectorAll(
          "#kanaRowCheckboxes input[type=checkbox]:checked",
        );
        return Array.from(checkboxes).map((cb) => cb.value);
      }

      function renderRoomList(rooms) {
        const list = $("roomList");
        const noRooms = $("noRooms");
        list.innerHTML = "";
        if (!rooms.length) {
          noRooms.classList.remove("hidden");
          return;
        }
        noRooms.classList.add("hidden");
        rooms.forEach((r) => {
          const li = document.createElement("li");
          li.className = "room-item fade-in";
          const statusLabel =
            r.status === "playing" ? "🎮 プレイ中" : "⏳ 待機中";
          const genreLabel =
            r.settings && r.settings.genre ? r.settings.genre : "なし";
          const playerCount =
            r.playerCount !== undefined ? r.playerCount : r.players || 0;
          const roomLink = getRoomLink(r.id);
          li.innerHTML = `
      <div class="room-info">
        <a class="room-name" href="${esc(roomLink)}" onclick="event.preventDefault(); showInviteRoomPreview('${esc(r.id)}');">${esc(r.name)}</a>
        <div class="room-meta">
          <span>👥 ${playerCount}人</span>
          <span>🏷️ ${esc(genreLabel)}</span>
          <span>${statusLabel}</span>
        </div>
      </div>
      <div class="room-actions">
        <div class="lobby-btn-wrap" data-lobby-btn>
          <button class="btn btn-primary" onclick="showInviteRoomPreview('${esc(r.id)}')"
            ${r.status === "playing" ? "data-playing disabled" : !$("playerName").value.trim() ? "disabled" : ""}>参加</button>
          <span class="lobby-btn-tooltip">${r.status === "playing" ? "プレイ中です" : "ユーザー名を入力してください"}</span>
        </div>
      </div>`;
          list.appendChild(li);
        });
      }

      function getName() {
        const name = $("playerName").value.trim();
        if (!name) {
          $("playerName").focus();
          $("playerName").style.borderColor = "var(--danger)";
          return null;
        }
        $("playerName").style.borderColor = "";
        return name;
      }

      function createRoom() {
        const name = getName();
        if (!name) return;
        myName = name;
        const allowedRows = getSelectedKanaRows();
        const noDakuten = $("noDakuten").checked;
        const isPrivate = $("privateRoom").checked;
        send({
          type: "create_room",
          name,
          settings: {
            name: $("roomNameInput").value.trim() || "しりとりルーム",
            minLen: parseInt($("minLen").value) || 1,
            maxLen: parseInt($("maxLen").value) || 0,
            genre: $("genre").value,
            timeLimit: parseInt($("timeLimit").value) || 0,
            maxLives: parseInt($("maxLives").value) || DEFAULT_MAX_LIVES,
            allowedRows: allowedRows.length > 0 ? allowedRows : undefined,
            noDakuten: noDakuten || undefined,
            private: isPrivate || undefined,
          },
        });
      }

      function joinRoom(roomId) {
        const name = getName();
        if (!name) return;
        myName = name;
        send({ type: "join", name, roomId });
        const url = new URL(window.location.href);
        url.searchParams.set("room", roomId);
        window.history.replaceState({}, "", url.toString());
        document.body.classList.remove("invite-lobby");
      }

      function buildRoomBadges(room) {
        const s = room.settings || {};
        let badges = [];
        if (s.private) badges.push("🔒 プライベート");
        if (room.owner) badges.push(`👑 ホスト: ${esc(room.owner)}`);
        if (s.genre) badges.push(`🏷️ ${esc(s.genre)}`);
        if (s.minLen > 1) badges.push(`最少${s.minLen}文字`);
        if (s.maxLen > 0) badges.push(`最大${s.maxLen}文字`);
        if (s.timeLimit > 0) badges.push(`⏱️ ${s.timeLimit}秒`);
        if (s.allowedRows && s.allowedRows.length > 0)
          badges.push(`🎯 ${s.allowedRows.map(esc).join("・")}`);
        if (s.noDakuten) badges.push("🚫 濁音・半濁音禁止");
        if (s.maxLives || s.maxLives === 0)
          badges.push(`❤️ ライフ${s.maxLives || DEFAULT_MAX_LIVES}`);
        const meta =
          room.playerCount !== undefined ? `👥 ${room.playerCount}人` : "";
        return [meta, ...badges]
          .filter(Boolean)
          .map((b) => `<span class="rule-badge">${b}</span>`)
          .join("");
      }

      function buildInviteBadges(room) {
        const badges = buildRoomBadges(room);
        return badges || '<span class="rule-badge">ルールなし</span>';
      }

      function renderRoomInfo(room) {
        if (!room) return;
        $("gameRoomTitle").textContent = room.name || "ルーム";
        $("gameRules").innerHTML = buildRoomBadges(room);
      }

      function renderInviteRoomInfo(room) {
        if (!room) return;
        $("inviteCardTitle").textContent = room.name || "招待ルーム";
        $("inviteCardRules").innerHTML = buildInviteBadges(room);
        $("inviteCardHost").textContent = room.owner
          ? `ホスト: ${room.owner}`
          : "";
      }

      function prepareWaitingRoom(room) {
        currentRoomId = room.id || "";
        currentSettings = room.settings || {};
        roomOwner = room.owner || "";
        showView("game");
        $("historyList").innerHTML = "";
        $("playerList").innerHTML = "";
        $("messageList").innerHTML = "";
        waitingPlayers = room.players || [];
        currentLives = {};
        maxLives = currentSettings.maxLives || DEFAULT_MAX_LIVES;
        currentTurn = "";
        turnOrder = [];
        isVoteActive = false;
        showActiveGame(false);
        $("playAgainBtn")?.classList.remove("hidden");
        $("playAgainWait")?.classList.add("hidden");
        $("gameOver").classList.add("hidden");
        renderRoomInfo(room);
        renderInviteRoomInfo(room);
        updateWaitingRoom();
      }

      /* ========== GAME ROOM ========== */
      function onJoined(msg) {
        currentRoomId = msg.roomId;
        currentSettings = msg.settings || {};
        roomOwner = msg.owner || "";
        currentLives = msg.lives || {};
        maxLives = msg.maxLives || currentSettings.maxLives || DEFAULT_MAX_LIVES;
        showView("game");
        $("gameRoomTitle").textContent = currentSettings.name || "ルーム";
        renderRules();
        renderPlayers(msg.players || [], msg.scores || {}, currentLives);
        $("historyList").innerHTML = "";
        (msg.history || []).forEach((h) => addHistoryItem(h.word, h.player));
        if (msg.turnOrder) turnOrder = msg.turnOrder;
        if (msg.currentTurn) currentTurn = msg.currentTurn;
        // Extract player names from players array
        waitingPlayers = (msg.players || []).map((p) =>
          typeof p === "object" ? p.name : p,
        );
        updateWaitingRoom();
        isVoteActive = false;
        updateMyLives();
        if (msg.currentWord && msg.status === "playing") {
          showActiveGame(true);
          setCurrentWord(msg.currentWord);
          updateTurnDisplay();
        } else {
          showActiveGame(false);
        }
        $("playAgainBtn")?.classList.remove("hidden");
        $("playAgainWait")?.classList.add("hidden");
        $("gameOver").classList.add("hidden");
        addMessage("ルームに参加しました", "info");

        if (inviteRoomId && inviteRoomId === currentRoomId) {
          clearInvite();
        }
        renderInviteRoomInfo({
          id: currentRoomId,
          name: currentSettings.name || "ルーム",
          owner: roomOwner,
          playerCount: waitingPlayers.length,
          settings: currentSettings,
          players: waitingPlayers,
        });
      }

      function onPlayerList(msg) {
        if (!currentRoomId) return;
        waitingPlayers = msg.players || [];
        updateWaitingRoom();
        renderInviteRoomInfo({
          id: currentRoomId,
          name: currentSettings.name || "ルーム",
          owner: roomOwner,
          playerCount: waitingPlayers.length,
          settings: currentSettings,
          players: waitingPlayers,
        });
      }

      function updateWaitingRoom() {
        const list = $("waitingPlayerList");
        if (!list) return;
        list.innerHTML = "";
        if (!waitingPlayers.length) {
          list.innerHTML = '<li class="waiting-empty">参加者なし</li>';
        } else {
          waitingPlayers.forEach((name) => {
            const li = document.createElement("li");
            li.innerHTML =
              esc(name) +
              (name === roomOwner
                ? ' <span class="owner-badge">ホスト</span>'
                : "") +
              (name === myName ? " 👈" : "");
            list.appendChild(li);
          });
        }
        // Show/hide start button based on ownership
        const isOwner = myName === roomOwner;
        $("startBtn").classList.toggle("hidden", !isOwner);
        $("waitOwnerText").classList.toggle("hidden", isOwner);
      }

      function buildRuleBadges(settings) {
        const s = settings || {};
        let badges = [];
        if (s.private) badges.push("🔒 プライベート");
        if (s.genre) badges.push(`🏷️ ${esc(s.genre)}`);
        if (s.minLen > 1) badges.push(`最少${s.minLen}文字`);
        if (s.maxLen > 0) badges.push(`最大${s.maxLen}文字`);
        if (s.timeLimit > 0) badges.push(`⏱️ ${s.timeLimit}秒`);
        if (s.allowedRows && s.allowedRows.length > 0)
          badges.push(`🎯 ${s.allowedRows.map(esc).join("・")}`);
        if (s.noDakuten) badges.push("🚫 濁音・半濁音禁止");
        if (s.maxLives || s.maxLives === 0)
          badges.push(`❤️ ライフ${s.maxLives || DEFAULT_MAX_LIVES}`);
        return badges
          .map((b) => `<span class="rule-badge">${b}</span>`)
          .join("");
      }

      function renderRules() {
        const s = currentSettings;
        $("gameRules").innerHTML = buildRuleBadges(s);
        timerMax = s.timeLimit || 0;
      }

      function onSettingsUpdated(msg) {
        currentSettings = msg.settings || currentSettings;
        renderRules();
        addMessage("⚙️ ルールが変更されました", "info");
      }

      function showActiveGame(active) {
        $("preGame").classList.toggle("hidden", active);
        $("activeGame").classList.toggle("hidden", !active);
        if (active) $("answerInput").focus();
      }

      function updateTurnDisplay() {
        const el = $("turnIndicator");
        const txt = $("turnText");
        const isMyTurn = currentTurn === myName;
        el.className =
          "turn-indicator " + (isMyTurn ? "my-turn" : "other-turn");
        txt.innerHTML = isMyTurn
          ? "🎯 あなたの番です！"
          : `⏳ ${esc(currentTurn)}さんの番です`;
        // Turn order badges
        if (turnOrder.length > 1) {
          const badges = turnOrder
            .map(
              (n) =>
                `<span class="turn-order-item${n === currentTurn ? " active" : ""}">${esc(n)}</span>`,
            )
            .join("");
          txt.innerHTML += `<div class="turn-order-list">${badges}</div>`;
        }
        // Enable/disable input
        const area = document.querySelector(".answer-area");
        if (area) area.classList.toggle("disabled", !isMyTurn);
        const input = $("answerInput");
        if (input) {
          input.value = "";
          input.disabled = !isMyTurn;
          const isFirstWord = $("currentWord").textContent === "―";
          input.placeholder = isMyTurn
            ? isFirstWord
              ? "最初のことばを入力…"
              : "ことばを入力…"
            : `${currentTurn}さんの番です…`;
          if (isMyTurn) input.focus();
        }
        const challengeBtn = $("challengeBtn");
        if (challengeBtn) {
          const canChallenge =
            $("currentWord").textContent !== "―" &&
            !isVoteActive &&
            lastWordPlayer !== myName;
          challengeBtn.disabled = !canChallenge;
        }
      }

      function setCurrentWord(word) {
        const el = $("currentWord");
        el.textContent = word || "―";
        el.style.animation = "none";
        el.offsetHeight;
        el.style.animation = "";
      }

      function renderPlayers(players, scores, lives) {
        const list = $("playerList");
        list.innerHTML = "";
        let items = [];
        if (
          Array.isArray(players) &&
          players.length > 0 &&
          typeof players[0] === "object"
        ) {
          items = players.map((p) => ({
            name: p.name,
            score: p.score || 0,
            lives: p.lives !== undefined ? p.lives : maxLives,
          }));
        } else {
          const arr = Array.isArray(players) ? players : Object.keys(players);
          items = arr.map((p) => ({
            name: p,
            score: (scores && scores[p]) || 0,
            lives: lives && lives[p] !== undefined ? lives[p] : maxLives,
          }));
        }
        items.sort((a, b) => b.score - a.score);
        items.forEach((p) => {
          const li = document.createElement("li");
          li.className = "player-item";
          if (p.lives <= 0) li.style.opacity = "0.4";
          const heartsStr =
            p.lives > 0 ? "❤️".repeat(Math.min(p.lives, 10)) : "💀";
          li.innerHTML = `<span class="player-name-display">${esc(p.name)}${p.name === myName ? " 👈" : ""}</span>
      <span class="player-lives">${heartsStr}</span>
      <span class="player-score">${p.score}点</span>`;
          list.appendChild(li);
        });
      }

      function onPlayerJoined(name) {
        addMessage(`${name}さんが参加しました`, "info");
        // Add to game player list
        const list = $("playerList");
        const exists = [...list.children].some((li) =>
          li.querySelector(".player-name-display").textContent.startsWith(name),
        );
        if (!exists) {
          const li = document.createElement("li");
          li.className = "player-item fade-in";
          li.innerHTML = `<span class="player-name-display">${esc(name)}</span><span class="player-score">0点</span>`;
          list.appendChild(li);
        }
        // Update waiting room
        if (!waitingPlayers.includes(name)) waitingPlayers.push(name);
        updateWaitingRoom();
      }

      function onTurnUpdate(msg) {
        if (msg.turnOrder) turnOrder = msg.turnOrder;
        if (msg.currentTurn) currentTurn = msg.currentTurn;
        if (msg.lives) currentLives = msg.lives;
        if (msg.maxLives) maxLives = msg.maxLives;
        updateTurnDisplay();
        renderPlayers(turnOrder, msg.scores || {}, currentLives);
      }

      function onPlayerLeft(name) {
        addMessage(`${name}さんが退出しました`, "error");
        const list = $("playerList");
        [...list.children].forEach((li) => {
          if (
            li
              .querySelector(".player-name-display")
              .textContent.startsWith(name)
          )
            li.remove();
        });
        // Update waiting room
        waitingPlayers = waitingPlayers.filter((n) => n !== name);
        updateWaitingRoom();
        if (currentRoomId && waitingPlayers.length === 0) {
          setTimeout(() => {
            if (currentRoomId && waitingPlayers.length === 0) {
              showView("lobby");
              refreshRooms();
            }
          }, 500);
        }
      }

      function startGame() {
        send({ type: "start_game" });
      }

      function onGameStarted(msg) {
        showActiveGame(true);
        $("historyList").innerHTML = "";
        setCurrentWord(msg.currentWord || msg.firstWord || "");
        if (msg.turnOrder) turnOrder = msg.turnOrder;
        if (msg.currentTurn) currentTurn = msg.currentTurn;
        if (msg.lives) currentLives = msg.lives;
        if (msg.maxLives) maxLives = msg.maxLives;
        isVoteActive = false;
        lastWordPlayer = "";
        updateMyLives();
        updateTurnDisplay();
        // Initialize timer display
        if (msg.timeLimit && msg.timeLimit > 0) {
          timerMax = msg.timeLimit;
          onTimer(msg.timeLimit);
        }
        if (msg.currentTurn === myName) {
          addMessage(
            "ゲームが始まりました！最初のことばを入力してください！",
            "success",
          );
        } else {
          addMessage(
            `ゲームが始まりました！${esc(msg.currentTurn)}さんが最初のことばを選びます`,
            "success",
          );
        }
        $("playAgainBtn")?.classList.remove("hidden");
        $("playAgainWait")?.classList.add("hidden");
        $("gameOver").classList.add("hidden");
      }

      function submitAnswer() {
        const input = $("answerInput");
        const word = input.value.trim();
        if (!word) return;
        send({ type: "answer", word });
        input.value = "";
        input.focus();
      }

      function onNewWord(msg) {
        setCurrentWord(msg.word);
        lastWordPlayer = msg.player;
        addHistoryItem(msg.word, msg.player);
        if (msg.lives) currentLives = msg.lives;
        if (msg.scores || msg.lives)
          updateScoresAndLives(msg.scores, msg.lives);
        if (msg.currentTurn) currentTurn = msg.currentTurn;
        updateMyLives();
        updateTurnDisplay();
        addMessage(`${msg.player}さんが正解！「${msg.word}」`, "success");
        showScorePopup(msg.player);
      }

      function addHistoryItem(word, player) {
        const list = $("historyList");
        const li = document.createElement("li");
        li.className = "history-item";
        li.innerHTML = `<span class="history-word">${esc(word)}</span>
    <span class="history-player">${esc(player)}</span>`;
        list.prepend(li);
      }

      function updateScores(scores) {
        updateScoresAndLives(scores, null);
      }

      function updateScoresAndLives(scores, lives) {
        if (lives) currentLives = lives;
        const list = $("playerList");
        const playerNames = [...list.children].map((li) =>
          li
            .querySelector(".player-name-display")
            .textContent.replace(" 👈", ""),
        );
        const items = playerNames.map((name) => ({
          name,
          score:
            scores && scores[name] !== undefined
              ? scores[name]
              : parseInt(
                  [...list.children]
                    .find(
                      (li) =>
                        li
                          .querySelector(".player-name-display")
                          .textContent.replace(" 👈", "") === name,
                    )
                    ?.querySelector(".player-score")?.textContent,
                ) || 0,
          lives:
            currentLives && currentLives[name] !== undefined
              ? currentLives[name]
              : maxLives,
        }));
        items.sort((a, b) => b.score - a.score);
        list.innerHTML = "";
        items.forEach((p) => {
          const li = document.createElement("li");
          li.className = "player-item";
          if (p.lives <= 0) li.style.opacity = "0.4";
          const heartsStr =
            p.lives > 0 ? "❤️".repeat(Math.min(p.lives, 10)) : "💀";
          li.innerHTML = `<span class="player-name-display">${esc(p.name)}${p.name === myName ? " 👈" : ""}</span>
      <span class="player-lives">${heartsStr}</span>
      <span class="player-score">${p.score}点</span>`;
          list.appendChild(li);
        });
      }

      function onInvalid(reason) {
        addMessage(reason, "error");
        const input = $("answerInput");
        input.style.animation = "shake .4s ease";
        input.style.borderColor = "var(--danger)";
        setTimeout(() => {
          input.style.animation = "";
          input.style.borderColor = "";
        }, 500);
      }

      function onTimer(seconds) {
        const section = $("timerSection");
        section.classList.remove("hidden");
        const text = $("timerText");
        const bar = $("timerBar");
        const pct = timerMax > 0 ? (seconds / timerMax) * 100 : 100;
        text.textContent = seconds + "秒";
        bar.style.width = pct + "%";
        text.className = "timer-text";
        bar.className = "timer-bar-inner";
        if (seconds <= 5) {
          text.classList.add("danger");
          bar.classList.add("danger");
        } else if (seconds <= 10) {
          text.classList.add("warning");
          bar.classList.add("warning");
        }
      }

      function onGameOver(msg) {
        const overlay = $("gameOver");
        overlay.classList.remove("hidden");
        let reason = msg.reason || "";
        if (msg.winner) {
          reason =
            `🏆 ${msg.winner}さんの勝利！` +
            (msg.loser ? ` (${msg.loser}さん脱落)` : "");
        } else if (msg.loser) {
          reason = `${msg.loser}さん - ${reason}`;
        }
        $("gameOverReason").textContent = reason;
        const list = $("finalScores");
        list.innerHTML = "";
        const scores = msg.scores || {};
        const sorted = Object.entries(scores).sort((a, b) => b[1] - a[1]);
        const medals = ["🥇", "🥈", "🥉"];
        sorted.forEach(([name, score], i) => {
          const li = document.createElement("li");
          li.className = "final-score-item";
          li.innerHTML = `<span class="final-rank">${medals[i] || i + 1}</span>
      <span class="final-name">${esc(name)}</span>
      <span class="final-pts">${score}点</span>`;
          list.appendChild(li);
        });
        clearInterval(timerInterval);
        $("timerSection").classList.add("hidden");

        // Render history in game-over card
        const history = msg.history || [];
        $("gameOverHistoryCount").textContent = history.length;
        const hList = $("gameOverHistoryList");
        hList.innerHTML = "";
        history.forEach((h, i) => {
          const li = document.createElement("li");
          li.innerHTML = `<span class="game-over-history-num">${i + 1}.</span>
      <span class="game-over-history-word">${esc(h.word)}</span>
      <span class="game-over-history-player">${esc(h.player)}</span>`;
          hList.appendChild(li);
        });
        // Show word chain summary
        const chain = history.map((h) => h.word).join(" → ");
        const chainEl = $("gameOverHistoryChain");
        if (history.length > 0) {
          chainEl.textContent = chain;
          chainEl.style.display = "";
        } else {
          chainEl.style.display = "none";
        }
        // Reset toggle state (collapsed)
        const body = $("gameOverHistoryBody");
        body.classList.remove("open");
        const toggle = body.previousElementSibling;
        toggle.classList.remove("open");

        // Show share section using server-provided result ID
        lastShareURL = "";
        $("shareSection").style.display = "none";
        const shareCopyBtn = $("shareCopyBtn");
        shareCopyBtn.classList.remove("copied");
        shareCopyBtn.innerHTML =
          '<span class="share-icon">\ud83d\udd17</span> \u30ea\u30f3\u30af\u30b3\u30d4\u30fc';

        if (msg.resultId) {
          lastShareURL = location.origin + "/results/" + msg.resultId;
          $("shareSection").style.display = "";
        } else {
          // Fallback: save via API (e.g. if server didn't include resultId)
          saveResultForSharing(msg);
        }

        // Populate settings panel for game-over screen
        populateGameOverSettings();
      }

      async function saveResultForSharing(msg) {
        try {
          const payload = {
            roomName: currentSettings.name || "\u3057\u308a\u3068\u308a",
            genre: currentSettings.genre || "",
            winner: msg.winner || "",
            reason: msg.reason || "",
            scores: msg.scores || {},
            history: msg.history || [],
            lives: msg.lives || {},
          };
          const resp = await fetch("/api/results", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(payload),
          });
          if (!resp.ok) throw new Error("save failed");
          const data = await resp.json();
          lastShareURL = location.origin + "/results/" + data.id;
          $("shareSection").style.display = "";
        } catch (e) {
          console.error("Failed to save result:", e);
        }
      }

      function buildShareText(msg) {
        const history = msg || [];
        const words =
          typeof history[0] === "object" ? history.map((h) => h.word) : [];
        // Use the stored game-over history from the DOM
        const hList = $("gameOverHistoryList");
        const wordEls = hList.querySelectorAll(".game-over-history-word");
        const ws = Array.from(wordEls).map((el) => el.textContent);
        const count = ws.length;

        let text = `\u3057\u308a\u3068\u308a\u3067${count}\u8a9e\u3064\u306a\u304e\u307e\u3057\u305f\uff01\n`;
        // Show chain (truncate for tweet)
        const chain = ws.join(" \u2192 ");
        const maxLen = 200;
        if (chain.length > maxLen) {
          text += chain.substring(0, maxLen) + "\u2026\n";
        } else {
          text += chain + "\n";
        }
        return text;
      }

      function shareToX() {
        if (!lastShareURL) return;
        const hList = $("gameOverHistoryList");
        const ws = Array.from(
          hList.querySelectorAll(".game-over-history-word"),
        ).map((el) => el.textContent);
        const count = ws.length;
        const chain = ws.join(" \u2192 ");
        let text = `\u3057\u308a\u3068\u308a\u3067${count}\u8a9e\u3064\u306a\u304e\u307e\u3057\u305f\uff01\n`;
        // Truncate chain to fit in tweet (280 - url - prefix ~ 200 chars for chain)
        if ([...chain].length > 140) {
          text += [...chain].slice(0, 137).join("") + "\u2026\n";
        } else {
          text += chain + "\n";
        }
        const url = `https://x.com/intent/tweet?text=${encodeURIComponent(text)}&url=${encodeURIComponent(lastShareURL)}`;
        window.open(url, "_blank", "width=550,height=420");
      }

      function shareToLINE() {
        if (!lastShareURL) return;
        const hList = $("gameOverHistoryList");
        const ws = Array.from(
          hList.querySelectorAll(".game-over-history-word"),
        ).map((el) => el.textContent);
        const count = ws.length;
        const chain = ws.join(" \u2192 ");
        let text = `\u3057\u308a\u3068\u308a\u3067${count}\u8a9e\u3064\u306a\u304e\u307e\u3057\u305f\uff01\n`;
        if ([...chain].length > 200) {
          text += [...chain].slice(0, 197).join("") + "\u2026\n";
        } else {
          text += chain + "\n";
        }
        text += lastShareURL;
        const url = `https://social-plugins.line.me/lineit/share?url=${encodeURIComponent(lastShareURL)}&text=${encodeURIComponent(text)}`;
        window.open(url, "_blank", "width=550,height=420");
      }

      async function shareLink() {
        if (!lastShareURL) return;
        try {
          await navigator.clipboard.writeText(lastShareURL);
          const btn = $("shareCopyBtn");
          btn.classList.add("copied");
          btn.innerHTML =
            '<span class="share-icon">\u2714</span> \u30b3\u30d4\u30fc\u3057\u307e\u3057\u305f';
          setTimeout(() => {
            btn.classList.remove("copied");
            btn.innerHTML =
              '<span class="share-icon">\ud83d\udd17</span> \u30ea\u30f3\u30af\u30b3\u30d4\u30fc';
          }, 2000);
        } catch {
          prompt("\u30ea\u30f3\u30af\u3092\u30b3\u30d4\u30fc:", lastShareURL);
        }
      }

      function toggleGameOverHistory(btn) {
        btn.classList.toggle("open");
        $("gameOverHistoryBody").classList.toggle("open");
      }

      function toggleGameOverSettings(btn) {
        btn.classList.toggle("open");
        $("gameOverSettingsBody").classList.toggle("open");
      }

      function populateGameOverSettings() {
        const s = currentSettings;
        $("goMinLen").value = s.minLen || 1;
        $("goMaxLen").value = s.maxLen || 0;
        $("goGenre").value = s.genre || "";
        // Set select values
        $("goTimeLimit").value = String(s.timeLimit || 0);
        $("goMaxLives").value = String(s.maxLives || DEFAULT_MAX_LIVES);
        $("goNoDakuten").checked = !!s.noDakuten;

        // Render kana row checkboxes for game-over settings
        const container = $("goKanaRowCheckboxes");
        if (container && kanaRowNames.length) {
          container.innerHTML = "";
          const allowed = s.allowedRows || [];
          kanaRowNames.forEach((name) => {
            const label = document.createElement("label");
            label.className = "kana-row-chip";
            const checked = allowed.includes(name);
            label.innerHTML = `<input type="checkbox" value="${esc(name)}"${checked ? " checked" : ""}> ${esc(name)}`;
            if (checked) label.classList.add("selected");
            label.addEventListener("click", () => {
              setTimeout(() => {
                const cb = label.querySelector("input");
                label.classList.toggle("selected", cb.checked);
                checkSettingsChanged();
              }, 0);
            });
            container.appendChild(label);
          });
        }

        // Listen for changes to show badge
        ["goMinLen", "goMaxLen", "goGenre", "goTimeLimit", "goMaxLives", "goNoDakuten"].forEach((id) => {
          const el = $(id);
          if (el) el.addEventListener("input", checkSettingsChanged);
          if (el) el.addEventListener("change", checkSettingsChanged);
        });

        // Show/hide settings panel (only for owner)
        $("gameOverSettings").style.display = (myName === roomOwner) ? "" : "none";
        // Reset changed badge
        $("settingsChangedBadge").classList.remove("visible");
        // Collapse panel
        $("gameOverSettingsBody").classList.remove("open");
        const toggle = $("gameOverSettingsBody").previousElementSibling;
        toggle.classList.remove("open");
      }

      function getGameOverSettings() {
        const goRows = Array.from(
          document.querySelectorAll("#goKanaRowCheckboxes input[type=checkbox]:checked")
        ).map((cb) => cb.value);
        return {
          name: currentSettings.name || "しりとりルーム",
          minLen: parseInt($("goMinLen").value) || 1,
          maxLen: parseInt($("goMaxLen").value) || 0,
          genre: $("goGenre").value,
          timeLimit: parseInt($("goTimeLimit").value) || 0,
          maxLives: parseInt($("goMaxLives").value) || DEFAULT_MAX_LIVES,
          allowedRows: goRows.length > 0 ? goRows : undefined,
          noDakuten: $("goNoDakuten").checked || undefined,
          private: currentSettings.private || undefined,
        };
      }

      function checkSettingsChanged() {
        const newS = getGameOverSettings();
        const s = currentSettings;
        const changed =
          newS.minLen !== (s.minLen || 1) ||
          newS.maxLen !== (s.maxLen || 0) ||
          newS.genre !== (s.genre || "") ||
          newS.timeLimit !== (s.timeLimit || 0) ||
          newS.maxLives !== (s.maxLives || DEFAULT_MAX_LIVES) ||
          !!newS.noDakuten !== !!s.noDakuten ||
          JSON.stringify(newS.allowedRows || []) !== JSON.stringify(s.allowedRows || []);
        $("settingsChangedBadge").classList.toggle("visible", changed);
        // Update button text
        $("playAgainBtn").textContent = changed ? "🔄 ルール変更して開始" : "🔄 もう一度";
      }

      function playAgain() {
        if (myName !== roomOwner) {
          $("playAgainBtn")?.classList.add("hidden");
          $("playAgainWait")?.classList.remove("hidden");
          addMessage("ホストの開始を待っています…", "info");
          return;
        }
        // Check if settings were changed
        const newS = getGameOverSettings();
        const s = currentSettings;
        const changed =
          newS.minLen !== (s.minLen || 1) ||
          newS.maxLen !== (s.maxLen || 0) ||
          newS.genre !== (s.genre || "") ||
          newS.timeLimit !== (s.timeLimit || 0) ||
          newS.maxLives !== (s.maxLives || DEFAULT_MAX_LIVES) ||
          !!newS.noDakuten !== !!s.noDakuten ||
          JSON.stringify(newS.allowedRows || []) !== JSON.stringify(s.allowedRows || []);
        if (changed) {
          send({ type: "start_game", settings: newS });
        } else {
          send({ type: "start_game" });
        }
      }

      function backToLobby() {
        $("gameOver").classList.add("hidden");
        leaveRoom();
      }

      function leaveRoom() {
        send({ type: "leave_room" });
        currentRoomId = "";
        showView("lobby");
        refreshRooms();
        document.body.classList.toggle("invite-lobby", !!inviteRoomId);
      }

      function handleInviteFromURL() {
        const params = new URLSearchParams(window.location.search);
        const roomId = params.get("room");
        if (roomId) {
          setInviteRoom(roomId);
          loadInviteRoomInfo(roomId, true);
        } else {
          $("inviteCard").classList.add("hidden");
          clearInviteRoomInfo();
          document.body.classList.remove("invite-lobby");
        }
      }

      function loadInviteRoomInfo(roomId, showPreviewOnly) {
        if (!roomId) return;
        fetch(`/room/${encodeURIComponent(roomId)}`)
          .then((res) => (res.ok ? res.json() : null))
          .then((data) => {
            if (!data) {
              clearInvite();
              showToast("ルームが存在しません", "error");
              return;
            }
            if (showPreviewOnly) {
              renderInviteRoomInfo(data);
              return;
            }
            prepareWaitingRoom(data);
            renderInviteRoomInfo(data);
          })
          .catch(() => {
            clearInvite();
            showToast("ルーム情報の取得に失敗しました", "error");
          });
      }

      function showInviteRoomPreview(roomId) {
        setInviteRoom(roomId);
        const url = new URL(window.location.href);
        url.searchParams.set("room", roomId);
        window.history.replaceState({}, "", url.toString());
        loadInviteRoomInfo(roomId, true);
      }

      /* ========== MESSAGES ========== */
      function showToast(message, type) {
        const container = $("toastContainer");
        const el = document.createElement("div");
        el.className = "toast" + (type ? " toast-" + type : "");
        el.textContent = message;
        container.appendChild(el);
        setTimeout(() => {
          el.classList.add("toast-out");
          el.addEventListener("animationend", () => el.remove());
        }, 3000);
      }

      function addMessage(text, type) {
        const list = $("messageList");
        if (!list) return;
        const li = document.createElement("li");
        li.className = "msg-item" + (type ? ` msg-${type}` : "");
        const now = new Date();
        const ts = `${String(now.getHours()).padStart(2, "0")}:${String(now.getMinutes()).padStart(2, "0")}`;
        li.textContent = `[${ts}] ${text}`;
        list.appendChild(li);
        list.scrollTop = list.scrollHeight;
      }

      /* ========== UI HELPERS ========== */
      function showView(view) {
        $("lobby").classList.toggle("hidden", view !== "lobby");
        $("game").classList.toggle("hidden", view !== "game");
        document.body.classList.toggle(
          "invite-view",
          view === "game" && !myName,
        );
        document.body.classList.toggle(
          "invite-lobby",
          view === "lobby" && !!inviteRoomId,
        );
      }

      function showScorePopup(player) {
        const el = document.createElement("div");
        el.className = "score-popup";
        el.textContent = "+1";
        el.style.left = Math.random() * 60 + 20 + "%";
        el.style.top = "40%";
        document.body.appendChild(el);
        setTimeout(() => el.remove(), 1000);
      }

      /* ========== VOTE ========== */
      let currentVotePlayerName = ""; // the challenged player in challenge votes

      function onVoteRequest(msg) {
        const isGenre = msg.voteType === "genre";
        const isChallenge = msg.voteType === "challenge";
        currentVotePlayerName = isChallenge ? msg.player : "";

        // Determine if this user is the challenged player
        const isChallengedPlayer = isChallenge && msg.player === myName;
        if (isChallenge) {
          hasVoted = msg.challenger === myName; // challenger auto-voted reject
        } else {
          hasVoted = msg.player === myName; // submitter auto-voted accept
        }
        $("voteTitle").textContent = isChallenge
          ? "🗳️ 単語指摘の投票"
          : "🗳️ ジャンル投票";
        $("voteWord").textContent = msg.word;
        if (isChallenge) {
          $("voteQuestion").textContent =
            `${msg.challenger}さんが「${msg.word}」を指摘しました`;
          $("voteGenreHint").textContent =
            msg.reason || "この単語を認めますか？";
        } else {
          $("voteQuestion").textContent =
            `${msg.player}さんが「${msg.word}」を入力しました`;
          $("voteGenreHint").textContent =
            `ジャンル「${msg.genre}」のリストにない単語です。認めますか？`;
        }
        updateVoteProgress(msg.voteCount || 0, msg.totalPlayers || 0);
        isVoteActive = true;
        $("voteOverlay").classList.remove("hidden");
        updateTurnDisplay();

        // Reset rebuttal display
        $("rebuttalDisplay").classList.add("hidden");
        $("rebuttalDisplay").innerHTML = "";

        // Determine if this user is the challenger
        const isChallenger = isChallenge && msg.challenger === myName;

        if (isChallengedPlayer) {
          // Challenged player: show rebuttal input instead of vote buttons
          $("voteButtonArea").classList.add("hidden");
          $("voteWaiting").classList.add("hidden");
          $("withdrawArea").classList.add("hidden");
          $("rebuttalArea").classList.remove("hidden");
          $("rebuttalInput").value = "";
          setTimeout(() => $("rebuttalInput").focus(), 100);
        } else if (hasVoted) {
          $("voteButtonArea").classList.add("hidden");
          $("rebuttalArea").classList.add("hidden");
          $("voteWaiting").classList.remove("hidden");
          // Show withdraw button for the challenger
          if (isChallenger) {
            $("withdrawArea").classList.remove("hidden");
          } else {
            $("withdrawArea").classList.add("hidden");
          }
        } else {
          $("voteButtonArea").classList.remove("hidden");
          $("rebuttalArea").classList.add("hidden");
          $("withdrawArea").classList.add("hidden");
          $("voteWaiting").classList.add("hidden");
        }

        // Vote countdown timer
        let voteTimeLeft = 15;
        $("voteTimer").textContent = `${voteTimeLeft}秒で自動判定`;
        clearInterval(voteTimerInterval);
        voteTimerInterval = setInterval(() => {
          voteTimeLeft--;
          if (voteTimeLeft <= 0) {
            clearInterval(voteTimerInterval);
            $("voteTimer").textContent = "判定中…";
          } else {
            $("voteTimer").textContent = `${voteTimeLeft}秒で自動判定`;
          }
        }, 1000);
      }

      function onVoteUpdate(msg) {
        updateVoteProgress(msg.voteCount || 0, msg.totalPlayers || 0);
      }

      function updateVoteProgress(count, total) {
        $("voteProgressText").textContent = `${count} / ${total} 投票済み`;
        const pct = total > 0 ? (count / total) * 100 : 0;
        $("voteProgressBar").style.width = pct + "%";
      }

      function onVoteResult(msg) {
        clearInterval(voteTimerInterval);
        isVoteActive = false;
        currentVotePlayerName = "";
        $("voteOverlay").classList.add("hidden");
        $("rebuttalArea").classList.add("hidden");
        $("rebuttalDisplay").classList.add("hidden");
        $("withdrawArea").classList.add("hidden");
        // Reset rebuttal input state
        const rebInput = $("rebuttalInput");
        rebInput.disabled = false;
        rebInput.placeholder = "反論を入力…";
        const rebBtn = rebInput.parentElement.querySelector(".btn");
        if (rebBtn) rebBtn.disabled = false;
        if (msg.accepted) {
          addMessage(
            msg.message || `投票により「${msg.word}」が承認されました`,
            "success",
          );
        } else {
          addMessage(
            msg.message || `投票により「${msg.word}」は却下されました`,
            "error",
          );
          if (msg.reverted) {
            if (msg.currentWord !== undefined)
              setCurrentWord(msg.currentWord || "");
            if (msg.history) {
              $("historyList").innerHTML = "";
              (msg.history || []).forEach((h) =>
                addHistoryItem(h.word, h.player),
              );
            }
            if (msg.scores || msg.lives)
              updateScoresAndLives(msg.scores, msg.lives);
            if (msg.currentTurn) currentTurn = msg.currentTurn;
            // Show penalty info for challenge rejection
            if (msg.penaltyPlayer) {
              if (msg.lives) currentLives = msg.lives;
              if (msg.eliminated) {
                addMessage(`💀 ${msg.penaltyPlayer}さんが脱落！`, "error");
                if (msg.penaltyPlayer === myName) {
                  addMessage("💀 あなたは脱落しました…", "error");
                }
              } else {
                addMessage(
                  `💔 ${msg.penaltyPlayer}さんがライフ-1！残り${msg.penaltyLives}`,
                  "error",
                );
                if (msg.penaltyPlayer === myName) {
                  document.body.style.animation = "shake .4s ease";
                  setTimeout(() => (document.body.style.animation = ""), 500);
                }
              }
            }
            updateMyLives();
            updateTurnDisplay();
          }
        }
        updateTurnDisplay();
      }

      function castVote(accept) {
        if (hasVoted) return;
        hasVoted = true;
        send({ type: "vote", accept });
        $("voteButtonArea").classList.add("hidden");
        $("voteWaiting").classList.remove("hidden");
      }

      function withdrawChallenge() {
        send({ type: "withdraw_challenge" });
      }

      function onChallengeWithdrawn(msg) {
        clearInterval(voteTimerInterval);
        isVoteActive = false;
        currentVotePlayerName = "";
        $("voteOverlay").classList.add("hidden");
        $("rebuttalArea").classList.add("hidden");
        $("rebuttalDisplay").classList.add("hidden");
        $("withdrawArea").classList.add("hidden");
        // Reset rebuttal input state
        const rebInput = $("rebuttalInput");
        rebInput.disabled = false;
        rebInput.placeholder = "反論を入力…";
        const rebBtn = rebInput.parentElement.querySelector(".btn");
        if (rebBtn) rebBtn.disabled = false;
        addMessage(msg.message || "指摘が取り下げられました", "info");
        updateTurnDisplay();
      }

      function sendRebuttal() {
        const input = $("rebuttalInput");
        const text = (input.value || "").trim();
        if (!text) return;
        send({ type: "rebuttal", rebuttal: text });
        input.value = "";
        input.placeholder = "送信済み ✓";
        input.disabled = true;
        input.parentElement.querySelector(".btn").disabled = true;
      }

      function onRebuttal(msg) {
        const display = $("rebuttalDisplay");
        const div = document.createElement("div");
        div.style.marginBottom = ".3rem";
        div.innerHTML = `<span class="rebuttal-sender">${esc(msg.player)}:</span> ${esc(msg.rebuttal)}`;
        display.appendChild(div);
        display.classList.remove("hidden");
        addMessage(`💬 ${msg.player}の反論: ${msg.rebuttal}`, "info");
      }

      function onPenalty(msg) {
        if (msg.allLives) currentLives = msg.allLives;
        updateScoresAndLives(null, msg.allLives);
        updateMyLives(msg.player === myName);

        const livesLeft = msg.lives !== undefined ? msg.lives : "?";
        if (msg.eliminated) {
          addMessage(`💀 ${msg.player}さんが脱落！「${msg.reason}」`, "error");
          if (msg.player === myName) {
            addMessage("💀 あなたは脱落しました…", "error");
          }
        } else {
          addMessage(
            `💔 ${msg.player}さんがライフ-1！残り${livesLeft} 「${msg.reason}」`,
            "error",
          );
          if (msg.player === myName) {
            document.body.style.animation = "shake .4s ease";
            setTimeout(() => (document.body.style.animation = ""), 500);
          }
        }
      }

      /* ========== MY LIVES DISPLAY ========== */
      function updateMyLives(animate) {
        const container = $("myLivesHearts");
        if (!container) return;
        const myLivesCount =
          currentLives && currentLives[myName] !== undefined
            ? currentLives[myName]
            : maxLives;
        container.innerHTML = "";
        for (let i = 0; i < maxLives; i++) {
          const span = document.createElement("span");
          span.className = "heart";
          if (i < myLivesCount) {
            span.textContent = "❤️";
          } else {
            span.textContent = "🤍";
            span.classList.add("lost");
            // Animate the most recently lost heart
            if (animate && i === myLivesCount) {
              span.classList.add("breaking");
            }
          }
          container.appendChild(span);
        }
      }

      function esc(str) {
        const d = document.createElement("div");
        d.textContent = str;
        return d.innerHTML;
      }

      function requestChallenge() {
        if (isVoteActive) return;
        if (lastWordPlayer === myName) {
          addMessage("自分の単語には指摘できません", "error");
          return;
        }
        send({ type: "challenge" });
      }

      /* ========== KEYBOARD ========== */
      $("answerInput")?.addEventListener("keydown", (e) => {
        if (e.key === "Enter") {
          e.preventDefault();
          submitAnswer();
        }
      });
      $("rebuttalInput")?.addEventListener("keydown", (e) => {
        if (e.key === "Enter") {
          e.preventDefault();
          sendRebuttal();
        }
      });

      document.addEventListener("keydown", (e) => {
        if (e.key === "Enter" && !$("lobby").classList.contains("hidden")) {
          return;
        }
      });

      /* ========== HIRAGANA-ONLY INPUT FILTER ========== */
      (function () {
        // Convert katakana to hiragana
        function katakanaToHiragana(str) {
          return str.replace(/[\u30A1-\u30F6]/g, function (ch) {
            return String.fromCharCode(ch.charCodeAt(0) - 0x60);
          });
        }
        // Check if a character is allowed: hiragana, katakana, or long vowel mark
        function isAllowedChar(ch) {
          var code = ch.charCodeAt(0);
          // Hiragana: U+3040-U+309F
          if (code >= 0x3040 && code <= 0x309f) return true;
          // Katakana: U+30A0-U+30FF
          if (code >= 0x30a0 && code <= 0x30ff) return true;
          // Long vowel mark
          if (ch === "\u30FC") return true;
          return false;
        }
        // Filter and convert input value
        function filterInput(input) {
          var pos = input.selectionStart;
          var original = input.value;
          // Keep only allowed characters
          var filtered = "";
          for (var i = 0; i < original.length; i++) {
            if (isAllowedChar(original[i])) filtered += original[i];
          }
          // Convert katakana to hiragana
          var converted = katakanaToHiragana(filtered);
          if (converted !== original) {
            var diff = original.length - converted.length;
            input.value = converted;
            // Adjust cursor position
            var newPos = Math.max(0, pos - diff);
            input.setSelectionRange(newPos, newPos);
          }
        }

        var answerInput = $("answerInput");
        if (answerInput) {
          var isComposing = false;
          answerInput.addEventListener("compositionstart", function () {
            isComposing = true;
          });
          answerInput.addEventListener("compositionend", function () {
            isComposing = false;
            filterInput(answerInput);
          });
          answerInput.addEventListener("input", function () {
            if (!isComposing) {
              filterInput(answerInput);
            }
          });
        }
      })();

      /* ========== INIT ========== */
      connect();
      handleInviteFromURL();
      window.addEventListener("popstate", handleInviteFromURL);
      // Refresh rooms periodically
      setInterval(() => {
        if (!$("lobby").classList.contains("hidden")) refreshRooms();
      }, 5000);

      // Toggle lobby buttons based on playerName input
      function updateLobbyButtons() {
        const hasName = $("playerName").value.trim().length > 0;
        document.querySelectorAll("[data-lobby-btn] .btn").forEach((btn) => {
          // Don't touch buttons disabled for other reasons (e.g. playing rooms)
          if (btn.hasAttribute("data-playing")) return;
          if (hasName) {
            btn.removeAttribute("disabled");
          } else {
            btn.setAttribute("disabled", "");
          }
        });
      }
      $("playerName").addEventListener("input", updateLobbyButtons);
      updateLobbyButtons();

      // ── Theme Switcher ──
      (function initThemeSwitcher() {
        var current = localStorage.getItem("shiritori-theme") || "default";
        var btns = document.querySelectorAll(".theme-btn");
        var cssLink = document.getElementById("themeCSS");
        var toggle = document.getElementById("themeToggle");
        var panel = document.getElementById("themePanel");

        function applyTheme(name) {
          document.documentElement.setAttribute("data-theme", name);
          localStorage.setItem("shiritori-theme", name);
          if (name === "default") {
            cssLink.href = "";
          } else {
            cssLink.href = "/static/themes/" + name + ".css";
          }
          btns.forEach(function (b) {
            b.classList.toggle("active", b.getAttribute("data-theme") === name);
          });
        }

        // Init active state
        btns.forEach(function (b) {
          if (b.getAttribute("data-theme") === current)
            b.classList.add("active");
          b.addEventListener("click", function () {
            applyTheme(this.getAttribute("data-theme"));
            panel.classList.remove("open");
          });
        });

        // Toggle panel
        toggle.addEventListener("click", function (e) {
          e.stopPropagation();
          panel.classList.toggle("open");
        });

        // Close on outside click
        document.addEventListener("click", function (e) {
          if (!panel.contains(e.target) && e.target !== toggle) {
            panel.classList.remove("open");
          }
        });
      })();
