var teams = []
var users = []

// Save attempts to POST or PUT to /teams/
function save(id) {
    var users = []
    $.each($("#usersTable").DataTable().rows().data(), function (i, user) {
        var $row = $("#usersTable").DataTable().row(i).node();
        var role = $($row).find('select#role').val();
        users.push({
            username: unescapeHtml(user[0]),
            id: parseInt(unescapeHtml(user[1])),
            role: role,
        })
    })
    var team = {
        name:  $("#name").val(),
        description: $("#description").val(),
        users: users
    }
    // Submit the team
    if (id !== -1) {
        // If we're just editing an existing team,
        // we need to PUT /teams/:id
        team.id = id
        api.teamId.put(team)
            .success(function (data) {
                successFlash("team updated successfully!")
                load()
                dismiss()
                $("#modal").modal('hide')
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    } else {
        // Else, if this is a new team, POST it
        // to /teams
        api.teams.post(team)
            .success(function (data) {
                successFlash("team added successfully!")
                load()
                dismiss()
                $("#modal").modal('hide')
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    }
}

function dismiss() {
    $("#usersTable").dataTable().DataTable().clear().draw()
    $("#name").val("")
    $("#modal\\.flashes").empty()
}

function edit(id) {
    setupUserOptions()

    users = $("#usersTable").dataTable({
        destroy: true, // Destroy any other instantiated table - http://datatables.net/manual/tech-notes/3#destroy
        columnDefs: [{
            orderable: false,
            users: "no-sort"
        }]
    })

    $("#modalSubmitTeam").unbind('click').click(function () {
        save(id)
    })
    if (id === -1) {
        $("#teamModalLabel").text("New team");
        var team = {}
    } else {
        $("#teamModalLabel").text("Edit team");
        api.teamId.get(id)
            .success(function (team) {
                $("#name").val(team.name)
                $("#description").val(team.description)
                userRows = []
                $.each(team.users, function (i, user) {
                    userRows.push([
                        escapeHtml(user.username),
                        escapeHtml(user.id),
                        '<select id="role" name="role_for">\
                        <option value="contributor"' + (user.role.slug == "contributor" ? "selected='selected'": "" ) + '>Contributor</option>\
                        <option value="viewer"' + (user.role.slug == "viewer" ? "selected='selected'": "" ) + '>Viewer</option>\
                        <option value="team_admin"' + (user.role.slug == "team_admin" ? "selected='selected'": "" ) + '>Team Leader</option>\
                        </select>',
                        '<span style="cursor:pointer;"><i class="fa fa-trash-o"></i></span>'
                    ])
                });
                users.DataTable().rows.add(userRows).draw()
            })
            .error(function () {
                errorFlash("Error fetching team")
            })
    }
}

var deleteTeam = function (id) {
    var team = teams.find(function (x) {
        return x.id === id
    })
    if (!team) {
        return
    }
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the team. This can't be undone!",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete " + escapeHtml(team.name),
        confirmButtonColor: "#10B981",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.teamId.delete(id)
                    .success(function (msg) {
                        resolve()
                    })
                    .error(function (data) {
                        reject(data.responseJSON.message)
                    })
            })
        }
    }).then(function (result) {
        if (result.value){
            Swal.fire(
                'team Deleted!',
                'This team has been deleted!',
                'success'
            );
        }
        $('button:contains("OK")').on('click', function () {
            location.reload()
        })
    })
}

function addUser(user) {
    // Create new data row.
    var username = escapeHtml(user.username).toLowerCase();
    var newRow = [
        escapeHtml(user.username),
        escapeHtml(user.id),
        '<select id="role" name="role_for" >\
        <option value="contributor">Contributor</option>\
        <option value="viewer" selected="selected">Viewer</option>\
        <option value="team_admin">Team Leader</option>\
        </select>',
        '<span style="cursor:pointer;"><i class="fa fa-trash-o"></i></span>'
    ];
    // Check table to see if email already exists.
    var existingRowIndex = users.DataTable()
        .column(0, {
            order: "index"
        }) // Email column has index of 2
        .data()
        .indexOf(username);
    // Update or add new row as necessary.
    if (existingRowIndex >= 0) {
        modalError("You can't assign a User twice")
    } else {
        users.DataTable().row.add(newRow);
    }
}

function setupUserOptions() {
    if (users.length === 0) {
        modalError("No users found!")
        return false;
    } else {
        var users_s2 = $.map(users, function (obj) {
            obj.text = obj.username
            return obj
        });
        $("#user.form-control").select2({
            placeholder: "Select User",
            data: users_s2,
            width: "200px",
        });
    }
}

function load() {
    $("#teamTable").hide()
    $("#emptyMessage").hide()
    $("#loading").show()
    api.users.get()
    .success(function (us) {
        users = us
    });
    api.teams.get()
        .success(function (response) {
            $("#loading").hide()
            if (response.length > 0) {
                teams = response
                $("#emptyMessage").hide()
                $("#teamTable").show()
                var teamTable = $("#teamTable").DataTable({
                    destroy: true,
                    columnDefs: [{
                        orderable: false,
                        targets: "no-sort"
                    }]
                });
                teamTable.clear();
                teamRows = []
                $.each(teams, function (i, team) {
                    teamRows.push([
                        escapeHtml(team.name),
                        escapeHtml(team.num_users),
                        "<div class='pull-right'><button class='btn btn-primary' data-toggle='modal' data-backdrop='static' data-target='#team_modal' onclick='edit(" + team.id + ")'>\
                    <i class='fa fa-pencil'></i>\
                    </button>\
                    <button class='btn btn-danger' onclick='deleteTeam(" + team.id + ")'>\
                    <i class='fa fa-trash-o'></i>\
                    </button></div>"
                    ])
                })
                teamTable.rows.add(teamRows).draw()
            } else {
                $("#emptyMessage").show()
            }
        })
        .error(function () {
            errorFlash("Error fetching teams")
        })
}

$(document).ready(function () {
    load()
    // Setup the event listeners
    // Handle manual additions
    $("#userForm").submit(function () {
        // Validate the form data
        var userForm = document.getElementById("userForm")
        if (!userForm.checkValidity()) {
            userForm.reportValidity()
            return
        }
        addUser(
            $("#user").select2("data")[0])
            users.DataTable().draw();

        // Reset user input.
        $("#userForm>div>input").val('');
        return false;
    });
    // Handle Deletion
    $("#usersTable").on("click", "span>i.fa-trash-o", function () {
        users.DataTable()
            .row($(this).parents('tr'))
            .remove()
            .draw();
    });
    $("#modal").on("hide.bs.modal", function () {
        dismiss();
    });
});
