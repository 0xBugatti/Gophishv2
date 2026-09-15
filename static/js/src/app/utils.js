// ------------------------------- HANDLE TEAMS -------------------------------------------

function setupTeamOptions() {
    if (teams.length === 0) {
        modalError("No teams found!")
        return false;
    } else {
        var teams_s2 = $.map(teams, function (obj) {
            obj.text = obj.name
            return obj
        });
        $("#team.form-control").select2({
            placeholder: "Select Teams",
            data: teams_s2,
        });
    }
}

function CheckTeam(teams, user) {
    var permissions = {
        canDelete : false,
        canEdit : false,
    };
    teams.forEach(team => {
        const teamRole = team.users.find(u => u.id === user.id);
        if (teamRole) {
          switch (teamRole.role.slug) {
            case 'team_admin':
                // Perform actions for team_admin
                console.log(`User is a Team Admin in team ${team.id}`);
                permissions.canDelete = true;
                permissions.canEdit = true;
                break;
            case 'contributor':
                // Perform actions for contributor
                console.log(`User is a Contributor in team ${team.id}`);
                permissions.canEdit = true;
                break;
            case 'viewer':
              // Perform actions for viewer
              console.log(`User is a Viewer in team ${team.id}`);
              break;

            default:
              console.log(`User ${user.user_id} has an unknown role in team ${team.id}`);
          }
        } else {
          console.log(`User ${user.user_id} is in team ${team.id} with no specified role`);
        }
      });
    return permissions
}

function addTeam(team) {
    // Create new data row.
    var newRow = [
        escapeHtml(team.text),
        escapeHtml(team.id),
        '<span style="cursor:pointer;"><i class="fa fa-trash-o"></i></span>'
    ];
    // Check table to see if email already exists.
    var existingRowIndex = teamTable
        .column(1, {
            order: "index"
        }) // Email column has index of 2
        .data()
        .indexOf(team.text);
    // Update or add new row as necessary.
    if (existingRowIndex >= 0) {
        teamTable
            .row(existingRowIndex, {
                order: "index"
            })
            .data(newRow);
    } else {
        teamTable.row.add(newRow);
    }
}

// Save attempts to POST to /teams/
function saveTeam(it, item_type, teams) {
    var ts = []
    $.each($("#teamTargetsTable").DataTable().rows().data(), function (i, t) {
        ts.push(teams.find(x => x.id == t[1]))
    })
    var item = {
        type: item_type,
        teams: ts
    }
    // Submit the team
    if (it.id !== -1) {
        // If we're just editing an existing team
        item.id = it.id
        api.item.post_teams(item)
            .success(function (data) {
                successFlash("Team updated successfully!")
                load()
                dismiss()
                $("#modal").modal('hide')
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    } else {
        // Else, if this is a not existing team
        modalError("Team doesn`t exist!")
    }
}

function updateItemTeamsAssignment(item, item_type, teams) {
    setupTeamOptions()
    $("#modalSubmitTeam").unbind('click').click(function () {
        saveTeam(item, item_type, teams)
    })
    teamTable = $("#teamTargetsTable").DataTable({
        destroy: true,
        columnDefs: [{
            orderable: false,
            targets: "no-sort"
        }]
    })
    teamTable.clear()
    teamRows = []
    if (item.teams.length === 0) {
        modalError("No teams assigned!")
        return false;
    } 
    $.each(item.teams, function (_, team) {
        teamRows.push([
            escapeHtml(team.name),
            escapeHtml(team.id),
            '<span style="cursor:pointer;"><i class="fa fa-trash-o"></i></span>'
        ])
    });
    teamTable.rows.add(teamRows).draw()
}

$(document).ready(function () {
    $("#teamTargetForm").submit(function () {   //team Form submit
        // Validate the form data
        var teamTargetForm = document.getElementById("teamTargetForm")
        if (!teamTargetForm.checkValidity()) {
            teamTargetForm.reportValidity()
            return
        }
        addTeam(
            $("#team").select2("data")[0])
            teamTable.draw();

        // Reset user input.
        $("#teamTargetForm>div>input").val('');
        return false;
    });
    // Handle Deletion of Team
    $("#teamTargetsTable").on("click", "span>i.fa-trash-o", function () {
        teamTable
            .row($(this).parents('tr'))
            .remove()
            .draw();
    });
})