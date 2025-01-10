/*******************************************
 *  Global Variables
 ********************************************/
let adminAuth = null; 
// We'll store { username, password } after logging in.

/*******************************************
 *  On Page Load
 ********************************************/
document.addEventListener('DOMContentLoaded', () => {
  // Fetch initial data
  fetchLeagues();
  fetchTeams();
  fetchSchedules();

  // Admin forms
  document.getElementById('createLeagueForm').addEventListener('submit', createLeague);
  document.getElementById('createTeamForm').addEventListener('submit', createTeam);
  document.getElementById('createScheduleForm').addEventListener('submit', createSchedule);
  document.getElementById('updateResultForm').addEventListener('submit', updateScheduleResult);
  document.getElementById('uploadLogoForm').addEventListener('submit', uploadLogo);

  // Admin login
  document.getElementById('loginBtn').addEventListener('click', doAdminLogin);

  // If previously stored admin credentials exist (not secure, just for demo)
  const storedUser = localStorage.getItem('adminUsername');
  const storedPass = localStorage.getItem('adminPassword');
  if (storedUser && storedPass) {
    adminAuth = { username: storedUser, password: storedPass };
    showSection('adminSection');
  }
});

/*******************************************
 *  Nav / Section Toggling
 ********************************************/
function showSection(sectionId) {
  const sections = document.querySelectorAll('main > section');
  sections.forEach(sec => {
    sec.id === sectionId ? sec.classList.add('active') : sec.classList.remove('active');
  });
}

function showAdminLogin() {
  // If already logged in, just show admin section
  if (adminAuth) {
    showSection('adminSection');
  } else {
    document.getElementById('loginOverlay').classList.add('active');
  }
}

function hideAdminLogin() {
  document.getElementById('loginOverlay').classList.remove('active');
}

/*******************************************
 *  Admin Login
 ********************************************/
function doAdminLogin() {
  const username = document.getElementById('adminUsername').value;
  const password = document.getElementById('adminPassword').value;

  if (!username || !password) {
    showError('loginError', 'Please enter username and password');
    return;
  }

  // We won't call a dedicated login endpoint. We'll just store credentials
  // and rely on Basic Auth for each admin request.
  adminAuth = { username, password };
  localStorage.setItem('adminUsername', username);
  localStorage.setItem('adminPassword', password);

  // Hide the login modal and show admin section
  hideAdminLogin();
  showSection('adminSection');
}

/*******************************************
 *  Fetch & Render: Leagues
 ********************************************/
function fetchLeagues() {
  fetch('/leagues')
    .then(res => res.json())
    .then(data => {
      const tbody = document.querySelector('#leaguesTable tbody');
      tbody.innerHTML = ''; // clear
      data.forEach(league => {
        const tr = document.createElement('tr');
        tr.innerHTML = `
          <td>${league.id}</td>
          <td>${league.name}</td>
          <td>${league.logo_url || ''}</td>
        `;
        tbody.appendChild(tr);
      });
    })
    .catch(err => console.error(err));
}

/*******************************************
 *  Fetch & Render: Teams
 ********************************************/
function fetchTeams() {
  fetch('/teams')
    .then(res => res.json())
    .then(data => {
      const tbody = document.querySelector('#teamsTable tbody');
      tbody.innerHTML = ''; // clear
      data.forEach(team => {
        const tr = document.createElement('tr');
        tr.innerHTML = `
          <td>${team.id}</td>
          <td>${team.league_id}</td>
          <td>${team.name}</td>
          <td>${team.elo_rating}</td>
          <td>
            <img 
              src="${team.logo_url || ''}" 
              alt="Logo" 
              style="max-width:50px;max-height:50px;"
            >
          </td>
        `;
        tbody.appendChild(tr);
      });
    })
    .catch(err => console.error(err));
}

/*******************************************
 *  Fetch & Render: Schedules
 ********************************************/
function fetchSchedules() {
  fetch('/schedules')
    .then(res => res.json())
    .then(data => {
      const tbody = document.querySelector('#schedulesTable tbody');
      tbody.innerHTML = ''; // clear
      data.forEach(sch => {
        const tr = document.createElement('tr');
        tr.innerHTML = `
          <td>${sch.id}</td>
          <td>${sch.league_id}</td>
          <td>${sch.home_team_id}</td>
          <td>${sch.away_team_id}</td>
          <td>${new Date(sch.match_date).toLocaleString()}</td>
          <td>${sch.status}</td>
          <td>${sch.home_score ?? ''}</td>
          <td>${sch.away_score ?? ''}</td>
        `;
        tbody.appendChild(tr);
      });
    })
    .catch(err => console.error(err));
}

/*******************************************
 *  Admin: Create League
 ********************************************/
function createLeague(e) {
  e.preventDefault();
  const name = document.getElementById('leagueName').value;
  const logo_url = document.getElementById('leagueLogoUrl').value;

  const payload = { name };
  if (logo_url) payload.logo_url = logo_url;

  fetch('/admin/leagues', {
    method: 'POST',
    headers: getAdminHeaders(),
    body: JSON.stringify(payload),
  })
    .then(res => {
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      return res.json();
    })
    .then(json => {
      showSuccess('leagueSuccess', json.message);
      fetchLeagues(); // refresh
    })
    .catch(err => showError('leagueError', `Error creating league: ${err.message}`));
}

/*******************************************
 *  Admin: Create Team
 ********************************************/
function createTeam(e) {
  e.preventDefault();
  const league_id = document.getElementById('teamLeagueId').value;
  const name = document.getElementById('teamName').value;
  const elo = document.getElementById('teamElo').value;

  const payload = { league_id, name };
  if (elo) payload.elo_rating = parseFloat(elo);

  fetch('/admin/teams', {
    method: 'POST',
    headers: getAdminHeaders(),
    body: JSON.stringify(payload),
  })
    .then(res => {
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      return res.json();
    })
    .then(json => {
      showSuccess('teamSuccess', json.message);
      fetchTeams(); // refresh
    })
    .catch(err => showError('teamError', `Error creating team: ${err.message}`));
}

/*******************************************
 *  Admin: Create Schedule
 ********************************************/
function createSchedule(e) {
  e.preventDefault();
  const league_id = document.getElementById('scheduleLeagueId').value;
  const home_team_id = document.getElementById('homeTeamId').value;
  const away_team_id = document.getElementById('awayTeamId').value;
  const matchDateInput = document.getElementById('matchDate').value;

  let match_date = null;
  if (matchDateInput) {
    // Convert local datetime to ISO
    match_date = new Date(matchDateInput).toISOString();
  }

  const payload = { league_id, home_team_id, away_team_id };
  if (match_date) payload.match_date = match_date;

  fetch('/admin/schedules', {
    method: 'POST',
    headers: getAdminHeaders(),
    body: JSON.stringify(payload),
  })
    .then(res => {
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      return res.json();
    })
    .then(json => {
      showSuccess('scheduleSuccess', json.message);
      fetchSchedules(); // refresh
    })
    .catch(err => showError('scheduleError', `Error creating schedule: ${err.message}`));
}

/*******************************************
 *  Admin: Update Schedule Result
 ********************************************/
function updateScheduleResult(e) {
  e.preventDefault();
  const scheduleId = document.getElementById('scheduleId').value;
  const homeScore = document.getElementById('homeScore').value;
  const awayScore = document.getElementById('awayScore').value;

  if (!scheduleId) {
    showError('resultError', 'Schedule ID is required');
    return;
  }

  const payload = {
    home_score: parseInt(homeScore),
    away_score: parseInt(awayScore),
  };

  fetch(`/admin/schedules/${scheduleId}/result`, {
    method: 'PUT',
    headers: getAdminHeaders(),
    body: JSON.stringify(payload),
  })
    .then(res => {
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      return res.json();
    })
    .then(json => {
      showSuccess('resultSuccess', json.message);
      fetchSchedules();
    })
    .catch(err => showError('resultError', `Error updating result: ${err.message}`));
}

/*******************************************
 *  Admin: Upload Logo
 ********************************************/
function uploadLogo(e) {
  e.preventDefault();
  const teamId = document.getElementById('logoTeamId').value;
  const fileInput = document.getElementById('logoFile');

  if (!teamId || !fileInput.files.length) {
    showError('logoError', 'Team ID and logo file are required');
    return;
  }

  const formData = new FormData();
  formData.append('logo', fileInput.files[0]);

  fetch(`/admin/teams/${teamId}/logo`, {
    method: 'POST',
    headers: getAdminHeaders(false), // false => do not set Content-Type for FormData
    body: formData,
  })
    .then(res => {
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      return res.json();
    })
    .then(json => {
      showSuccess('logoSuccess', json.message);
      fetchTeams(); // reload teams to see new logo
    })
    .catch(err => showError('logoError', `Error uploading logo: ${err.message}`));
}

/*******************************************
 *  Utility: Admin Headers for Basic Auth
 ********************************************/
function getAdminHeaders(json = true) {
  if (!adminAuth) {
    return json
      ? { 'Content-Type': 'application/json' }
      : {};
  }
  const token = btoa(`${adminAuth.username}:${adminAuth.password}`);
  const headers = {
    Authorization: `Basic ${token}`
  };
  if (json) {
    headers['Content-Type'] = 'application/json';
  }
  return headers;
}

/*******************************************
 *  Utility: Show/Hide success/error messages
 ********************************************/
function showSuccess(id, msg) {
  const el = document.getElementById(id);
  el.classList.remove('hidden');
  el.textContent = msg;
  // Hide after 3 seconds
  setTimeout(() => {
    el.classList.add('hidden');
  }, 3000);
}

function showError(id, msg) {
  const el = document.getElementById(id);
  el.classList.remove('hidden');
  el.textContent = msg;
  // Hide after 5 seconds
  setTimeout(() => {
    el.classList.add('hidden');
  }, 5000);
}